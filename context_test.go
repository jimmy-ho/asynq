// Copyright 2020 Kentaro Hibino. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package asynq

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hibiken/asynq/internal/base"
	"github.com/rs/xid"
)

func TestCreateContextWithTimeRestrictions(t *testing.T) {
	var (
		noTimeout  = time.Duration(0)
		noDeadline = time.Time{}
	)

	tests := []struct {
		desc         string
		timeout      time.Duration
		deadline     time.Time
		wantDeadline time.Time
	}{
		{"only with timeout", 10 * time.Second, noDeadline, time.Now().Add(10 * time.Second)},
		{"only with deadline", noTimeout, time.Now().Add(time.Hour), time.Now().Add(time.Hour)},
		{"with timeout and deadline (timeout < deadline)", 10 * time.Second, time.Now().Add(time.Hour), time.Now().Add(10 * time.Second)},
		{"with timeout and deadline (timeout > deadline)", 10 * time.Minute, time.Now().Add(30 * time.Second), time.Now().Add(30 * time.Second)},
	}

	for _, tc := range tests {
		msg := &base.TaskMessage{
			Type:     "something",
			ID:       xid.New(),
			Timeout:  tc.timeout.String(),
			Deadline: tc.deadline.Format(time.RFC3339),
		}

		ctx, cancel := createContext(msg)

		select {
		case x := <-ctx.Done():
			t.Errorf("%s: <-ctx.Done() == %v, want nothing (it should block)", tc.desc, x)
		default:
		}

		got, ok := ctx.Deadline()
		if !ok {
			t.Errorf("%s: ctx.Deadline() returned false, want deadline to be set", tc.desc)
		}
		if !cmp.Equal(tc.wantDeadline, got, cmpopts.EquateApproxTime(time.Second)) {
			t.Errorf("%s: ctx.Deadline() returned %v, want %v", tc.desc, got, tc.wantDeadline)
		}

		cancel()

		select {
		case <-ctx.Done():
		default:
			t.Errorf("ctx.Done() blocked, want it to be non-blocking")
		}
	}
}

func TestCreateContextWithoutTimeRestrictions(t *testing.T) {
	msg := &base.TaskMessage{
		Type:     "something",
		ID:       xid.New(),
		Timeout:  time.Duration(0).String(),        // zero value to indicate no timeout
		Deadline: time.Time{}.Format(time.RFC3339), // zero value to indicate no deadline
	}

	ctx, cancel := createContext(msg)

	select {
	case x := <-ctx.Done():
		t.Errorf("<-ctx.Done() == %v, want nothing (it should block)", x)
	default:
	}

	_, ok := ctx.Deadline()
	if ok {
		t.Error("ctx.Deadline() returned true, want deadline to not be set")
	}

	cancel()

	select {
	case <-ctx.Done():
	default:
		t.Error("ctx.Done() blocked, want it to be non-blocking")
	}
}

func TestGetTaskMetadataFromContext(t *testing.T) {
	tests := []struct {
		desc string
		msg  *base.TaskMessage
	}{
		{"with zero retried message", &base.TaskMessage{Type: "something", ID: xid.New(), Retry: 25, Retried: 0}},
		{"with non-zero retried message", &base.TaskMessage{Type: "something", ID: xid.New(), Retry: 10, Retried: 5}},
	}

	for _, tc := range tests {
		ctx, _ := createContext(tc.msg)

		id, ok := GetTaskID(ctx)
		if !ok {
			t.Errorf("%s: GetTaskID(ctx) returned ok == false", tc.desc)
		}
		if ok && id != tc.msg.ID.String() {
			t.Errorf("%s: GetTaskID(ctx) returned id == %q, want %q", tc.desc, id, tc.msg.ID.String())
		}

		retried, ok := GetRetryCount(ctx)
		if !ok {
			t.Errorf("%s: GetRetryCount(ctx) returned ok == false", tc.desc)
		}
		if ok && retried != tc.msg.Retried {
			t.Errorf("%s: GetRetryCount(ctx) returned n == %d want %d", tc.desc, retried, tc.msg.Retried)
		}

		maxRetry, ok := GetMaxRetry(ctx)
		if !ok {
			t.Errorf("%s: GetMaxRetry(ctx) returned ok == false", tc.desc)
		}
		if ok && maxRetry != tc.msg.Retry {
			t.Errorf("%s: GetMaxRetry(ctx) returned n == %d want %d", tc.desc, maxRetry, tc.msg.Retry)
		}
	}
}

func TestGetTaskMetadataFromContextError(t *testing.T) {
	tests := []struct {
		desc string
		ctx  context.Context
	}{
		{"with background context", context.Background()},
	}

	for _, tc := range tests {
		if _, ok := GetTaskID(tc.ctx); ok {
			t.Errorf("%s: GetTaskID(ctx) returned ok == true", tc.desc)
		}
		if _, ok := GetRetryCount(tc.ctx); ok {
			t.Errorf("%s: GetRetryCount(ctx) returned ok == true", tc.desc)
		}
		if _, ok := GetMaxRetry(tc.ctx); ok {
			t.Errorf("%s: GetMaxRetry(ctx) returned ok == true", tc.desc)
		}
	}
}
