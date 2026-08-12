package recovery

import (
	"fmt"
	"time"

	"discrescue/internal/mapfile"
	"discrescue/internal/recoverymap"
)

const (
	MaxPendingRecoveredBytes = uint64(8 << 20)
	MaxPendingRecords        = uint32(256)
	MaxCheckpointInterval    = time.Second
)

type CheckpointReason string

const (
	CheckpointReasonThreshold      CheckpointReason = "threshold"
	CheckpointReasonInterval       CheckpointReason = "interval"
	CheckpointReasonPassTransition CheckpointReason = "pass-transition"
	CheckpointReasonRecoveryReturn CheckpointReason = "recovery-return"
	CheckpointReasonStop           CheckpointReason = "stop"
)

type CheckpointStats struct {
	Count                     uint64
	LastReason                CheckpointReason
	LastPendingRecoveredBytes uint64
	LastPendingRecords        uint32
	LastImageSyncDuration     time.Duration
	LastMapSyncDuration       time.Duration
}

type recoveryPersistence struct {
	output recoverySyncWriter
	store  *recoverymap.Store
	now    func() time.Time

	pendingRecoveredBytes uint64
	pendingRecords        uint32
	lastCheckpoint        time.Time
	stats                 CheckpointStats
}

func NewPersistence(output SyncWriterAt, store *recoverymap.Store) *recoveryPersistence {
	now := time.Now
	return &recoveryPersistence{output: output, store: store, now: now, lastCheckpoint: now()}
}

func (p *recoveryPersistence) Extents() []mapfile.Extent {
	return p.store.Extents()
}

func (p *recoveryPersistence) DurableExtents() []mapfile.Extent {
	return p.store.DurableExtents()
}

func (p *recoveryPersistence) ApplyExtent(extent mapfile.Extent) error {
	if err := p.ForceCheckpoint(CheckpointReasonPassTransition); err != nil {
		return err
	}
	return p.store.ApplyExtent(extent)
}

func (p *recoveryPersistence) PersistRecovered(data []byte, offset int64, extent mapfile.Extent) error {
	if len(data) == 0 {
		return fmt.Errorf("persist recovered data: empty sector payload")
	}
	if uint64(len(data)) > MaxPendingRecoveredBytes {
		return fmt.Errorf("persist recovered data: payload %d exceeds pending-byte limit %d", len(data), MaxPendingRecoveredBytes)
	}
	if p.pendingRecoveredBytes > 0 && p.pendingRecoveredBytes+uint64(len(data)) > MaxPendingRecoveredBytes {
		if err := p.ForceCheckpoint(CheckpointReasonThreshold); err != nil {
			return err
		}
	}
	if err := writeFullAtWriter(p.output, data, offset); err != nil {
		return fmt.Errorf("write recovered data at byte %d: %w", offset, err)
	}
	if err := p.store.StageExtent(extent); err != nil {
		return fmt.Errorf("stage recovered extent [%d,%d): %w", extent.StartLBA, extent.EndLBA(), err)
	}
	p.pendingRecoveredBytes += uint64(len(data))
	p.pendingRecords++
	if p.pendingRecoveredBytes >= MaxPendingRecoveredBytes || p.pendingRecords >= MaxPendingRecords {
		return p.ForceCheckpoint(CheckpointReasonThreshold)
	}
	if p.now().Sub(p.lastCheckpoint) >= MaxCheckpointInterval {
		return p.ForceCheckpoint(CheckpointReasonInterval)
	}
	return nil
}

func (p *recoveryPersistence) MaybeCheckpoint(reason CheckpointReason) error {
	if p.pendingRecoveredBytes == 0 && p.store.PendingBytes() == 0 {
		return nil
	}
	return p.ForceCheckpoint(reason)
}

func (p *recoveryPersistence) ForceCheckpoint(reason CheckpointReason) error {
	if p.pendingRecoveredBytes == 0 && p.store.PendingBytes() == 0 {
		p.lastCheckpoint = p.now()
		return nil
	}
	imageStart := time.Now()
	if p.pendingRecoveredBytes > 0 {
		if err := p.output.Sync(); err != nil {
			return fmt.Errorf("checkpoint image sync: %w", err)
		}
	}
	imageDuration := time.Since(imageStart)
	mapStart := time.Now()
	if err := p.store.Flush(); err != nil {
		return fmt.Errorf("checkpoint map sync: %w", err)
	}
	mapDuration := time.Since(mapStart)
	p.stats.Count++
	p.stats.LastReason = reason
	p.stats.LastPendingRecoveredBytes = p.pendingRecoveredBytes
	p.stats.LastPendingRecords = p.pendingRecords
	p.stats.LastImageSyncDuration = imageDuration
	p.stats.LastMapSyncDuration = mapDuration
	p.pendingRecoveredBytes = 0
	p.pendingRecords = 0
	p.lastCheckpoint = p.now()
	return nil
}

func (p *recoveryPersistence) Stats() CheckpointStats {
	return p.stats
}

func (p *recoveryPersistence) Flush() error {
	return p.ForceCheckpoint(CheckpointReasonRecoveryReturn)
}
