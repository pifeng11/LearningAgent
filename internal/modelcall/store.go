package modelcall

import (
	"context"

	"learning-agent/internal/model"
)

type Store interface {
	model.ModelCallRecorder
	ListModelCalls(ctx context.Context, query model.ModelCallQuery) ([]model.ModelCall, error)
	GetModelCall(ctx context.Context, id int64) (model.ModelCall, bool, error)
}

type NoopStore struct{}

func (NoopStore) CreateModelCall(ctx context.Context, call model.ModelCall) (model.ModelCall, error) {
	return call, nil
}

func (NoopStore) CompleteModelCall(ctx context.Context, id int64, update model.ModelCallUpdate) error {
	return nil
}

func (NoopStore) ListModelCalls(ctx context.Context, query model.ModelCallQuery) ([]model.ModelCall, error) {
	return []model.ModelCall{}, nil
}

func (NoopStore) GetModelCall(ctx context.Context, id int64) (model.ModelCall, bool, error) {
	return model.ModelCall{}, false, nil
}
