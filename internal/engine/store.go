package engine

import (
	"context"
	"encoding/json"
)

const dealStateKey = "deal_state"

func (e *Engine) loadState(ctx context.Context) (bool, error) {
	if e.store == nil {
		return false, nil
	}
	data, ok, err := e.store.Get(ctx, dealStateKey)
	if err != nil || !ok {
		return false, err
	}
	var state DealState
	if err := json.Unmarshal(data, &state); err != nil {
		return false, err
	}
	if !state.Active {
		return false, nil
	}
	e.mu.Lock()
	e.state = state
	e.ensureStateMaps()
	e.mu.Unlock()
	return true, nil
}

func (e *Engine) saveState(ctx context.Context) {
	if e.store == nil {
		return
	}
	e.mu.Lock()
	data, err := json.Marshal(e.state)
	e.mu.Unlock()
	if err != nil {
		e.logEntry().WithError(err).Warn("Не удалось сериализовать состояние.")
		return
	}
	if err := e.store.Put(ctx, dealStateKey, data); err != nil {
		e.logEntry().WithError(err).Warn("Не удалось сохранить состояние.")
	}
}

func (e *Engine) clearState(ctx context.Context) {
	if e.store == nil {
		return
	}
	if err := e.store.Delete(ctx, dealStateKey); err != nil {
		e.logEntry().WithError(err).Warn("Не удалось удалить состояние.")
	}
}
