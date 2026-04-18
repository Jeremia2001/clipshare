package services

import (
	"context"
	"errors"

	"clipshare/internal/repository"
)

var ErrServerStorageFull = errors.New("server storage limit reached")

type InstanceService struct {
	repo repository.InstanceRepository
}

func NewInstanceService(repo repository.InstanceRepository) *InstanceService {
	return &InstanceService{repo: repo}
}

type StorageStatus struct {
	LimitBytes int64 `json:"limit_bytes"` // 0 means unlimited
	UsedBytes  int64 `json:"used_bytes"`
}

func (s *InstanceService) GetStorageStatus(ctx context.Context) (*StorageStatus, error) {
	limit, err := s.repo.GetStorageLimit(ctx)
	if err != nil {
		return nil, err
	}
	used, err := s.repo.GetTotalStorageUsed(ctx)
	if err != nil {
		return nil, err
	}
	return &StorageStatus{LimitBytes: limit, UsedBytes: used}, nil
}

func (s *InstanceService) SetStorageLimit(ctx context.Context, bytes int64) error {
	return s.repo.SetStorageLimit(ctx, bytes)
}

// CheckRoomFor returns ErrServerStorageFull if accepting `size` more bytes would
// push the instance past its limit. A limit of 0 disables the check.
func (s *InstanceService) CheckRoomFor(ctx context.Context, size int64) error {
	limit, err := s.repo.GetStorageLimit(ctx)
	if err != nil {
		return err
	}
	if limit <= 0 {
		return nil
	}
	used, err := s.repo.GetTotalStorageUsed(ctx)
	if err != nil {
		return err
	}
	if used+size > limit {
		return ErrServerStorageFull
	}
	return nil
}
