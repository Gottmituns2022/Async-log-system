package main

import (
	"golang.org/x/net/context"
	"sync"
	"time"
)

type BizManager struct{}

func NewBizManager() *BizManager {
	return &BizManager{}
}
func (bm *BizManager) mustEmbedUnimplementedBizServer() {}

func (bm *BizManager) Check(context.Context, *Nothing) (*Nothing, error) {
	return &Nothing{Dummy: true}, nil
}

func (bm *BizManager) Add(context.Context, *Nothing) (*Nothing, error) {
	return &Nothing{Dummy: true}, nil
}

func (bm *BizManager) Test(context.Context, *Nothing) (*Nothing, error) {
	return &Nothing{Dummy: true}, nil
}

func (am *AdminManager) mustEmbedUnimplementedAdminServer() {}

type AdminManager struct {
	SubscribersLog  map[chan *Event]struct{}
	SubscribersStat map[chan *Event]struct{}
	Mu              sync.Mutex
}

func NewAdminManager() *AdminManager {
	return &AdminManager{
		SubscribersLog:  make(map[chan *Event]struct{}),
		SubscribersStat: make(map[chan *Event]struct{}),
	}
}

func (am *AdminManager) Logging(nt *Nothing, inStream Admin_LoggingServer) error {
	logSubscriber := make(chan *Event, 100)

	am.Mu.Lock()
	am.SubscribersLog[logSubscriber] = struct{}{}
	am.Mu.Unlock()

	defer func() {
		am.Mu.Lock()
		delete(am.SubscribersLog, logSubscriber)
		am.Mu.Unlock()
		close(logSubscriber)
	}()

	for event := range logSubscriber {
		if err := inStream.Send(event); err != nil {
			return err
		}
	}
	return nil
}

func (am *AdminManager) Statistics(interval *StatInterval, inStream Admin_StatisticsServer) error {
	ticker := time.NewTicker(time.Duration(interval.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	statSubscriber := make(chan *Event, 100)

	am.Mu.Lock()
	am.SubscribersStat[statSubscriber] = struct{}{}
	am.Mu.Unlock()

	byMethodMap := make(map[string]uint64)
	byConsumerMap := make(map[string]uint64)

	defer func() {
		am.Mu.Lock()
		delete(am.SubscribersLog, statSubscriber)
		am.Mu.Unlock()
		close(statSubscriber)
	}()

	for {
		select {
		case event := <-statSubscriber:
			am.Mu.Lock()
			byMethodMap[event.Method]++
			byConsumerMap[event.Consumer]++
			am.Mu.Unlock()
		case <-ticker.C:
			stat := &Stat{
				ByMethod:   byMethodMap,
				ByConsumer: byConsumerMap,
				Timestamp:  time.Now().Unix(),
			}
			err := inStream.Send(stat)
			if err != nil {
				return err
			}
			byMethodMap = make(map[string]uint64)
			byConsumerMap = make(map[string]uint64)
		case <-inStream.Context().Done():
			return nil
		}
	}
}
