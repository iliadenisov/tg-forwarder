package registry

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type FwdHandler func(srcChan int64, msgId []int)

type Registry struct {
	ctx     context.Context
	handler FwdHandler
	cache   map[int64]map[int64]msgGroup
	lock    sync.Mutex
}

type msgGroup struct {
	GroupingId int64
	MessageIds []int
	RandomIds  []int64
	Ts         int
}

func NewRegistry(ctx context.Context) *Registry {
	return &Registry{
		ctx:     ctx,
		cache:   make(map[int64]map[int64]msgGroup),
		handler: func(srcChan int64, msgId []int) {},
	}
}

func (r *Registry) OnMessageForward(handler FwdHandler) {
	r.handler = handler
}

func (r *Registry) RegisterMessage(channelId, groupingId int64, messageId, ts int) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if _, ok := r.cache[channelId]; !ok {
		r.cache[channelId] = make(map[int64]msgGroup)
		defer func() {
			go r.processMessages(channelId)
		}()
	}

	var mg msgGroup
	if _, ok := r.cache[channelId][groupingId]; ok {
		mg = r.cache[channelId][groupingId]
	} else {
		mg = msgGroup{GroupingId: groupingId, Ts: ts}
	}
	mg.MessageIds = append(mg.MessageIds, messageId)
	r.cache[channelId][groupingId] = mg
}

func (r *Registry) processMessages(channelId int64) {
	for {
		t := time.After(5 * time.Second)
		select {
		case <-r.ctx.Done():
			return
		case <-t:
			r.lock.Lock()
		}

		if _, ok := r.cache[channelId]; !ok || len(r.cache[channelId]) == 0 {
			delete(r.cache, channelId)
			break
		} else {
			mg := []msgGroup{}
			for _, v := range r.cache[channelId] {
				mg = append(mg, v)
			}

			slices.SortFunc(mg, func(a, b msgGroup) int { return a.Ts - b.Ts })

			groupingId := mg[0].GroupingId

			if i, ok := r.cache[channelId][groupingId]; ok {
				delete(r.cache[channelId], groupingId)
				go r.handler(channelId, i.MessageIds)
			}
		}

		r.lock.Unlock()
	}
}

func GetForwardMap(env string) (map[int64]int64, error) {
	result := make(map[int64]int64)
	v, ok := os.LookupEnv(env)
	if !ok {
		return nil, fmt.Errorf("environment variable %s not set", env)
	}
	groups := strings.Split(v, "|")
	if len(groups) == 0 {
		return nil, fmt.Errorf("channel groups is empty, check channel groups delimiter at environment variable %s", env)
	}
	for _, g := range groups {
		gr := strings.Split(g, ":")
		if len(gr) != 2 {
			return nil, fmt.Errorf("channel group %q is empty, check source-target delimiter", g)
		}
		dstCh, err := strconv.Atoi(gr[0])
		if err != nil {
			return nil, fmt.Errorf("group %q destination channel_id should be an integer, %q given", g, gr[0])
		}
		src := strings.Split(gr[1], ",")
		for i := range src {
			srcCh, err := strconv.Atoi(src[i])
			if err != nil {
				return nil, fmt.Errorf("group %q source channel_id should be an integer, %q given", g, src[i])
			}
			if d, ok := result[int64(srcCh)]; ok {
				return nil, fmt.Errorf("group %q source channel_id %d already in use for destination channel_id %d", g, srcCh, d)
			}
			if d, ok := result[int64(dstCh)]; ok {
				return nil, fmt.Errorf("group %q destination channel_id %d already in use as source channel_id %d, cyclic forwards not supported", g, srcCh, d)
			}
			result[int64(srcCh)] = int64(dstCh)
		}
	}
	return result, nil
}
