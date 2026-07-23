package main

import (
	"sync/atomic"
	"time"
)

type State int32

const (
	StateClosed   State = 0
	StateOpen     State = 1
	StateHalfOpen State = 2
)

type CircuitBreaker struct {
	state           atomic.Int32
	failureCount    atomic.Int64
	successCount    atomic.Int64
	threshold       int64
	halfOpenMax     int64
	openTimeout     time.Duration
	lastStateChange atomic.Int64
}

func NewCircuitBreaker(threshold int64, halfOpenMax int64, openTimeout time.Duration) *CircuitBreaker {
	cb := &CircuitBreaker{
		threshold:   threshold,
		halfOpenMax: halfOpenMax,
		openTimeout: openTimeout,
	}
	cb.state.Store(int32(StateClosed))
	return cb
}

func (cb *CircuitBreaker) State() State {
	return State(cb.state.Load())
}

func (cb *CircuitBreaker) Allow() bool {
	state := cb.State()
	switch state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(time.Unix(0, cb.lastStateChange.Load())) > cb.openTimeout {
			cb.state.CompareAndSwap(int32(StateOpen), int32(StateHalfOpen))
			cb.successCount.Store(0)
			return true
		}
		return false
	case StateHalfOpen:
		return cb.successCount.Load() < cb.halfOpenMax
	default:
		return true
	}
}

func (cb *CircuitBreaker) Success() {
	state := cb.State()
	switch state {
	case StateHalfOpen:
		count := cb.successCount.Add(1)
		if count >= cb.halfOpenMax {
			cb.reset()
		}
	case StateClosed:
		cb.failureCount.Store(0)
	}
}

func (cb *CircuitBreaker) Failure() {
	state := cb.State()
	switch state {
	case StateClosed:
		count := cb.failureCount.Add(1)
		if count >= cb.threshold {
			cb.trip()
		}
	case StateHalfOpen:
		cb.trip()
	}
}

func (cb *CircuitBreaker) reset() {
	cb.state.Store(int32(StateClosed))
	cb.failureCount.Store(0)
	cb.successCount.Store(0)
}

func (cb *CircuitBreaker) trip() {
	cb.state.Store(int32(StateOpen))
	cb.lastStateChange.Store(time.Now().UnixNano())
	cb.failureCount.Store(0)
	cb.successCount.Store(0)
}
