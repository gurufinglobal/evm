package stream

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCondBroadcastDuringUnlock(t *testing.T) {
	cond := NewCond()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Force a producer notification immediately after the reader releases its
	// condition lock, before it starts waiting. No later broadcast can rescue it.
	require.True(t, cond.wait(ctx, cond.Broadcast))
}

func TestCondCanceledWait(t *testing.T) {
	cond := NewCond()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	unlockCalled := false
	require.False(t, cond.wait(ctx, func() { unlockCalled = true }))
	require.True(t, unlockCalled)
	require.False(t, cond.Wait(ctx))
}
