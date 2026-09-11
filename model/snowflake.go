package model

import (
	"sync"
	"time"
)

const (
	sfWorkerBits  uint8 = 10
	sfNumberBits  uint8 = 12
	sfWorkerMax   int64 = -1 ^ (-1 << sfWorkerBits)
	sfNumberMax   int64 = -1 ^ (-1 << sfNumberBits)
	sfTimeShift   uint8 = sfWorkerBits + sfNumberBits
	sfWorkerShift uint8 = sfNumberBits
	sfStartTime   int64 = 1704067200000 // 2024-01-01 00:00:00 UTC
)

type snowflake struct {
	mu        sync.Mutex
	timestamp int64
	workerID  int64
	number    int64
}

var sf *snowflake

func InitSnowflake(workerID int64) {
	if workerID < 0 || workerID > sfWorkerMax {
		workerID = 0
	}
	sf = &snowflake{workerID: workerID}
}

func NextID() int64 {
	if sf == nil {
		InitSnowflake(0)
	}
	sf.mu.Lock()
	defer sf.mu.Unlock()

	now := time.Now().UnixMilli()
	if sf.timestamp == now {
		sf.number++
		if sf.number > sfNumberMax {
			for now <= sf.timestamp {
				now = time.Now().UnixMilli()
			}
			sf.number = 0
			sf.timestamp = now
		}
	} else {
		sf.number = 0
		sf.timestamp = now
	}
	return (now-sfStartTime)<<sfTimeShift | (sf.workerID << sfWorkerShift) | sf.number
}
