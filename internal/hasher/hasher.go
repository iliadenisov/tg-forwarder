package hasher

import (
	"context"
	"sync"
)

type hasher struct {
	accHash map[int64]int64
	lock    sync.RWMutex
}

func NewHasher() *hasher {
	return &hasher{
		accHash: make(map[int64]int64),
	}
}

func (h *hasher) SetChannelAccessHash(ctx context.Context, userID, channelID, accessHash int64) error {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.accHash[channelID] = accessHash
	return nil
}

func (h *hasher) GetChannelAccessHash(ctx context.Context, userID, channelID int64) (accessHash int64, found bool, err error) {
	h.lock.RLock()
	defer h.lock.RUnlock()

	if v, ok := h.accHash[channelID]; !ok {
		return v, false, nil
	} else {
		return v, true, nil
	}
}
