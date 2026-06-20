package server

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/apernet/quic-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/goleak"

	"github.com/apernet/hysteria/core/v2/internal/frag"
	"github.com/apernet/hysteria/core/v2/internal/protocol"
	"github.com/apernet/hysteria/core/v2/internal/utils"
)

func TestUDPSessionManager(t *testing.T) {
	io := newMockUDPIO(t)
	eventLogger := newMockUDPEventLogger(t)
	sm := newUDPSessionManager(io, eventLogger, 2*time.Second, 1, 1)

	msgCh := make(chan *protocol.UDPMessage, 4)
	io.EXPECT().ReceiveMessage().RunAndReturn(func() (*protocol.UDPMessage, error) {
		m := <-msgCh
		if m == nil {
			return nil, errors.New("closed")
		}
		return m, nil
	})

	go sm.Run()

	udpReadFunc := func(addr string, ch chan []byte, b []byte) (int, string, error) {
		bs := <-ch
		if bs == nil {
			return 0, "", errors.New("closed")
		}
		n := copy(b, bs)
		return n, addr, nil
	}

	// Test normal session creation & timeout
	msg1 := &protocol.UDPMessage{
		SessionID: 1234,
		PacketID:  0,
		FragID:    0,
		FragCount: 1,
		Addr:      "address1.com:9000",
		Data:      []byte("hello"),
	}
	eventLogger.EXPECT().New(msg1.SessionID, msg1.Addr).Return().Once()
	udpConn1 := newMockUDPConn(t)
	udpConn1Ch := make(chan []byte, 1)
	io.EXPECT().Hook(msg1.Data, &msg1.Addr).Return(nil).Once()
	io.EXPECT().UDP(msg1.Addr).Return(udpConn1, nil).Once()
	udpConn1.EXPECT().WriteTo(msg1.Data, msg1.Addr).Return(5, nil).Once()
	udpConn1.EXPECT().ReadFrom(mock.Anything).RunAndReturn(func(b []byte) (int, string, error) {
		return udpReadFunc(msg1.Addr, udpConn1Ch, b)
	})
	io.EXPECT().LogReceive(len("hi back")).Return(nil).Once()
	io.EXPECT().SendMessage(mock.Anything, &protocol.UDPMessage{
		SessionID: msg1.SessionID,
		PacketID:  0,
		FragID:    0,
		FragCount: 1,
		Addr:      msg1.Addr,
		Data:      []byte("hi back"),
	}).Return(nil).Once()
	msgCh <- msg1
	udpConn1Ch <- []byte("hi back")

	msg2data := []byte("how are you doing?")
	msg2_1 := &protocol.UDPMessage{
		SessionID: 5678,
		PacketID:  0,
		FragID:    0,
		FragCount: 2,
		Addr:      "address2.net:12450",
		Data:      msg2data[:6],
	}
	msg2_2 := &protocol.UDPMessage{
		SessionID: 5678,
		PacketID:  0,
		FragID:    1,
		FragCount: 2,
		Addr:      "address2.net:12450",
		Data:      msg2data[6:],
	}

	eventLogger.EXPECT().New(msg2_1.SessionID, msg2_1.Addr).Return().Once()
	udpConn2 := newMockUDPConn(t)
	udpConn2Ch := make(chan []byte, 1)
	// On fragmentation, make sure hook gets the whole message
	io.EXPECT().Hook(msg2data, &msg2_1.Addr).Return(nil).Once()
	io.EXPECT().UDP(msg2_1.Addr).Return(udpConn2, nil).Once()
	udpConn2.EXPECT().WriteTo(msg2data, msg2_1.Addr).Return(11, nil).Once()
	udpConn2.EXPECT().ReadFrom(mock.Anything).RunAndReturn(func(b []byte) (int, string, error) {
		return udpReadFunc(msg2_1.Addr, udpConn2Ch, b)
	})
	io.EXPECT().LogReceive(len("im fine")).Return(nil).Once()
	io.EXPECT().SendMessage(mock.Anything, &protocol.UDPMessage{
		SessionID: msg2_1.SessionID,
		PacketID:  0,
		FragID:    0,
		FragCount: 1,
		Addr:      msg2_1.Addr,
		Data:      []byte("im fine"),
	}).Return(nil).Once()
	msgCh <- msg2_1
	msgCh <- msg2_2
	udpConn2Ch <- []byte("im fine")

	msg3 := &protocol.UDPMessage{
		SessionID: 1234,
		PacketID:  0,
		FragID:    0,
		FragCount: 1,
		Addr:      "address1.com:9000",
		Data:      []byte("who are you?"),
	}
	udpConn1.EXPECT().WriteTo(msg3.Data, msg3.Addr).Return(12, nil).Once()
	io.EXPECT().LogReceive(len("im your father")).Return(nil).Once()
	io.EXPECT().SendMessage(mock.Anything, &protocol.UDPMessage{
		SessionID: msg3.SessionID,
		PacketID:  0,
		FragID:    0,
		FragCount: 1,
		Addr:      msg3.Addr,
		Data:      []byte("im your father"),
	}).Return(nil).Once()
	msgCh <- msg3
	udpConn1Ch <- []byte("im your father")

	// Make sure timeout works (connections closed & close events emitted)
	udpConn1.EXPECT().Close().RunAndReturn(func() error {
		close(udpConn1Ch)
		return nil
	}).Once()
	udpConn2.EXPECT().Close().RunAndReturn(func() error {
		close(udpConn2Ch)
		return nil
	}).Once()
	eventLogger.EXPECT().Close(msg1.SessionID, nil).Once()
	eventLogger.EXPECT().Close(msg2_1.SessionID, nil).Once()

	time.Sleep(3 * time.Second) // Wait for timeout
	mock.AssertExpectationsForObjects(t, io, eventLogger, udpConn1, udpConn2)

	// Test UDP connection close error propagation
	errUDPClosed := errors.New("UDP connection closed")
	msg4 := &protocol.UDPMessage{
		SessionID: 666,
		PacketID:  0,
		FragID:    0,
		FragCount: 1,
		Addr:      "oh-no.com:27015",
		Data:      []byte("dont say bye"),
	}
	eventLogger.EXPECT().New(msg4.SessionID, msg4.Addr).Return().Once()
	udpConn4 := newMockUDPConn(t)
	io.EXPECT().Hook(msg4.Data, &msg4.Addr).Return(nil).Once()
	io.EXPECT().UDP(msg4.Addr).Return(udpConn4, nil).Once()
	udpConn4.EXPECT().WriteTo(msg4.Data, msg4.Addr).Return(12, nil).Once()
	udpConn4.EXPECT().ReadFrom(mock.Anything).Return(0, "", errUDPClosed).Once()
	udpConn4.EXPECT().Close().Return(nil).Once()
	eventLogger.EXPECT().Close(msg4.SessionID, errUDPClosed).Once()
	msgCh <- msg4

	time.Sleep(1 * time.Second)
	mock.AssertExpectationsForObjects(t, io, eventLogger, udpConn4)

	// Test UDP connection creation error propagation
	errUDPIO := errors.New("UDP IO error")
	msg5 := &protocol.UDPMessage{
		SessionID: 777,
		PacketID:  0,
		FragID:    0,
		FragCount: 1,
		Addr:      "callmemaybe.com:15353",
		Data:      []byte("babe i miss you"),
	}
	eventLogger.EXPECT().New(msg5.SessionID, msg5.Addr).Return().Once()
	io.EXPECT().Hook(msg5.Data, &msg5.Addr).Return(nil).Once()
	io.EXPECT().UDP(msg5.Addr).Return(nil, errUDPIO).Once()
	eventLogger.EXPECT().Close(msg5.SessionID, errUDPIO).Once()
	msgCh <- msg5

	time.Sleep(1 * time.Second)
	mock.AssertExpectationsForObjects(t, io, eventLogger)

	// Leak checks
	close(msgCh)                // This will return error from ReceiveMessage(), should stop the session manager
	time.Sleep(1 * time.Second) // Wait one more second just to be sure
	assert.Zero(t, sm.Count(), "session count should be 0")
	goleak.VerifyNone(t)
}

func TestUDPSessionEntryFeedRedundantWriteTo(t *testing.T) {
	for _, tc := range []struct {
		name       string
		multiplier int
	}{
		{name: "one", multiplier: 1},
		{name: "three", multiplier: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg := &protocol.UDPMessage{
				SessionID: 1,
				FragCount: 1,
				Addr:      "example.com:443",
				Data:      []byte("hello"),
			}
			conn := newMockUDPConn(t)
			conn.EXPECT().WriteTo(msg.Data, msg.Addr).Return(len(msg.Data), nil).Times(tc.multiplier)
			entry := &udpSessionEntry{
				D:                               &frag.Defragger{},
				Last:                            utils.NewAtomicTime(time.Now()),
				conn:                            conn,
				aclCache:                        map[string]error{msg.Addr: nil},
				writeToRedundancyMultiplier:     tc.multiplier,
				sendMessageRedundancyMultiplier: 1,
			}

			n, err := entry.Feed(msg)

			assert.NoError(t, err)
			assert.Equal(t, len(msg.Data), n)
		})
	}
}

func TestUDPSessionManagerClosesOnRedundantWriteError(t *testing.T) {
	io := newMockUDPIO(t)
	eventLogger := newMockUDPEventLogger(t)
	sm := newUDPSessionManager(io, eventLogger, 30*time.Second, 3, 1)

	msg := &protocol.UDPMessage{
		SessionID: 42,
		FragCount: 1,
		Addr:      "target.example:9000",
		Data:      []byte("payload"),
	}
	sendErr := errors.New("write failed")
	readCh := make(chan []byte)
	readDone := make(chan struct{})

	eventLogger.EXPECT().New(msg.SessionID, msg.Addr).Return().Once()
	io.EXPECT().Hook(msg.Data, &msg.Addr).Return(nil).Once()
	conn := newMockUDPConn(t)
	io.EXPECT().UDP(msg.Addr).Return(conn, nil).Once()

	writeCount := 0
	conn.EXPECT().WriteTo(msg.Data, msg.Addr).RunAndReturn(func([]byte, string) (int, error) {
		writeCount++
		if writeCount == 2 {
			return 0, sendErr
		}
		return len(msg.Data), nil
	}).Times(2)
	conn.EXPECT().ReadFrom(mock.Anything).RunAndReturn(func(b []byte) (int, string, error) {
		defer close(readDone)
		bs := <-readCh
		if bs == nil {
			return 0, "", errors.New("closed")
		}
		return copy(b, bs), msg.Addr, nil
	})
	conn.EXPECT().Close().RunAndReturn(func() error {
		close(readCh)
		return nil
	}).Once()
	eventLogger.EXPECT().Close(msg.SessionID, sendErr).Return().Once()

	sm.feed(msg)

	assert.Equal(t, 2, writeCount)
	assert.Zero(t, sm.Count())
	select {
	case <-readDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for receive loop to exit")
	}
}

func TestSendMessageAutoFragWithRedundancy(t *testing.T) {
	t.Run("unfragmented", func(t *testing.T) {
		io := newMockUDPIO(t)
		msg := &protocol.UDPMessage{
			SessionID: 7,
			FragCount: 1,
			Addr:      "reply.example:5353",
			Data:      []byte("reply"),
		}
		io.EXPECT().SendMessage(mock.Anything, mock.MatchedBy(func(got *protocol.UDPMessage) bool {
			return got.SessionID == msg.SessionID &&
				got.PacketID == 0 &&
				got.FragID == 0 &&
				got.FragCount == 1 &&
				got.Addr == msg.Addr &&
				bytes.Equal(got.Data, msg.Data)
		})).Return(nil).Times(3)

		err := sendMessageAutoFragWithRedundancy(io, make([]byte, protocol.MaxUDPSize), msg, 3)

		assert.NoError(t, err)
	})

	t.Run("fragmented complete rounds", func(t *testing.T) {
		io := newMockUDPIO(t)
		msg := &protocol.UDPMessage{
			SessionID: 8,
			FragCount: 1,
			Addr:      "reply.example:5353",
			Data:      bytes.Repeat([]byte("x"), 96),
		}
		maxDatagramPayloadSize := msg.HeaderSize() + 32
		expectedFrags := frag.FragUDPMessage(&protocol.UDPMessage{
			SessionID: msg.SessionID,
			PacketID:  1,
			FragID:    0,
			FragCount: 1,
			Addr:      msg.Addr,
			Data:      msg.Data,
		}, maxDatagramPayloadSize)

		fullAttempts := 0
		var rounds [][]protocol.UDPMessage
		var currentRound []protocol.UDPMessage
		io.EXPECT().SendMessage(mock.Anything, mock.Anything).RunAndReturn(func(_ []byte, got *protocol.UDPMessage) error {
			if got.FragCount == 1 && bytes.Equal(got.Data, msg.Data) {
				fullAttempts++
				if len(currentRound) > 0 {
					rounds = append(rounds, currentRound)
					currentRound = nil
				}
				return &quic.DatagramTooLargeError{MaxDatagramPayloadSize: int64(maxDatagramPayloadSize)}
			}
			currentRound = append(currentRound, *got)
			return nil
		}).Times((len(expectedFrags) + 1) * 2)

		err := sendMessageAutoFragWithRedundancy(io, make([]byte, protocol.MaxUDPSize), msg, 2)

		assert.NoError(t, err)
		if len(currentRound) > 0 {
			rounds = append(rounds, currentRound)
		}
		assert.Equal(t, 2, fullAttempts)
		assert.Len(t, rounds, 2)
		for _, round := range rounds {
			assert.Len(t, round, len(expectedFrags))
			packetID := round[0].PacketID
			assert.NotZero(t, packetID)
			for i, got := range round {
				assert.Equal(t, packetID, got.PacketID)
				assert.Equal(t, expectedFrags[i].FragID, got.FragID)
				assert.Equal(t, expectedFrags[i].FragCount, got.FragCount)
				assert.Equal(t, expectedFrags[i].Addr, got.Addr)
				assert.Equal(t, expectedFrags[i].Data, got.Data)
			}
		}
	})

	t.Run("returns repeat error", func(t *testing.T) {
		io := newMockUDPIO(t)
		msg := &protocol.UDPMessage{
			SessionID: 9,
			FragCount: 1,
			Addr:      "reply.example:5353",
			Data:      []byte("reply"),
		}
		sendErr := errors.New("send failed")
		sendCount := 0
		io.EXPECT().SendMessage(mock.Anything, mock.Anything).RunAndReturn(func([]byte, *protocol.UDPMessage) error {
			sendCount++
			if sendCount == 2 {
				return sendErr
			}
			return nil
		}).Times(2)

		err := sendMessageAutoFragWithRedundancy(io, make([]byte, protocol.MaxUDPSize), msg, 3)

		assert.ErrorIs(t, err, sendErr)
		assert.Equal(t, 2, sendCount)
	})
}

func TestUDPSessionEntryReceiveLoopClosesOnRedundantSendError(t *testing.T) {
	io := newMockUDPIO(t)
	conn := newMockUDPConn(t)
	sendErr := errors.New("send failed")
	errCh := make(chan error, 1)
	entry := &udpSessionEntry{
		ID:   77,
		Last: utils.NewAtomicTime(time.Now()),
		IO:   io,
		conn: conn,
		ExitFunc: func(err error) {
			errCh <- err
		},
		writeToRedundancyMultiplier:     1,
		sendMessageRedundancyMultiplier: 3,
	}
	reply := []byte("reply")
	addr := "target.example:9000"

	conn.EXPECT().ReadFrom(mock.Anything).RunAndReturn(func(b []byte) (int, string, error) {
		return copy(b, reply), addr, nil
	}).Once()
	io.EXPECT().LogReceive(len(reply)).Return(nil).Once()
	sendCount := 0
	io.EXPECT().SendMessage(mock.Anything, mock.Anything).RunAndReturn(func([]byte, *protocol.UDPMessage) error {
		sendCount++
		if sendCount == 2 {
			return sendErr
		}
		return nil
	}).Times(2)
	conn.EXPECT().Close().Return(nil).Once()

	go entry.receiveLoop()

	assert.ErrorIs(t, <-errCh, sendErr)
	assert.Equal(t, 2, sendCount)
}

func TestUDPIOImplLogsTrafficWithSplitRedundancyMultipliers(t *testing.T) {
	logger := &recordingTrafficLogger{}
	io := &udpIOImpl{
		AuthID:                          "user-1",
		TrafficLogger:                   logger,
		WriteToRedundancyMultiplier:     3,
		SendMessageRedundancyMultiplier: 4,
	}

	assert.NoError(t, io.LogTransmit(5))
	assert.NoError(t, io.LogReceive(7))
	assert.Equal(t, []trafficLogEntry{
		{id: "user-1", tx: 15, rx: 0},
		{id: "user-1", tx: 0, rx: 28},
	}, logger.entries)
}

func TestUDPIOImplLogsTrafficDefaultMultiplierAsOne(t *testing.T) {
	logger := &recordingTrafficLogger{}
	io := &udpIOImpl{
		AuthID:        "user-1",
		TrafficLogger: logger,
	}

	assert.NoError(t, io.LogTransmit(5))
	assert.NoError(t, io.LogReceive(7))
	assert.Equal(t, []trafficLogEntry{
		{id: "user-1", tx: 5, rx: 0},
		{id: "user-1", tx: 0, rx: 7},
	}, logger.entries)
}

type trafficLogEntry struct {
	id string
	tx uint64
	rx uint64
}

type recordingTrafficLogger struct {
	entries []trafficLogEntry
}

func (l *recordingTrafficLogger) LogTraffic(id string, tx, rx uint64) bool {
	l.entries = append(l.entries, trafficLogEntry{id: id, tx: tx, rx: rx})
	return true
}

func (l *recordingTrafficLogger) LogOnlineState(string, bool) {}

func (l *recordingTrafficLogger) TraceStream(HyStream, *StreamStats) {}

func (l *recordingTrafficLogger) UntraceStream(HyStream) {}
