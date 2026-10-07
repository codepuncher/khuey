// Package nightlight suspends the KDE Plasma night light while a sync session
// is running, through KWin's NightLight inhibition interface.
package nightlight

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	kwinService = "org.kde.KWin"
	kwinPath    = "/org/kde/KWin/NightLight"
	kwinIface   = "org.kde.KWin.NightLight"

	// Acquire and Release run under the sync engine's lock, so a wedged KWin
	// must not hold it indefinitely.
	kwinCallTimeout = 2 * time.Second
)

// Reason names a kind of sync session that suspends the night light.
type Reason int

const (
	ReasonSync Reason = iota
	ReasonGaming
)

// Inhibitor holds a single KWin inhibition while any reason is held. The
// inhibition is taken with the first reason and released with the last.
type Inhibitor struct {
	inhibit   func(ctx context.Context) (uint32, error)
	uninhibit func(ctx context.Context, cookie uint32) error

	mu     sync.Mutex
	held   map[Reason]bool
	cookie uint32
}

/**
 * New returns an Inhibitor that talks to KWin over conn.
 */
func New(conn *dbus.Conn) *Inhibitor {
	obj := conn.Object(kwinService, kwinPath)
	return newInhibitor(
		func(ctx context.Context) (uint32, error) {
			var cookie uint32
			err := obj.CallWithContext(ctx, kwinIface+".inhibit", 0).Store(&cookie)
			return cookie, err
		},
		func(ctx context.Context, cookie uint32) error {
			return obj.CallWithContext(ctx, kwinIface+".uninhibit", 0, cookie).Err
		},
	)
}

func newInhibitor(
	inhibit func(context.Context) (uint32, error),
	uninhibit func(context.Context, uint32) error,
) *Inhibitor {
	return &Inhibitor{
		inhibit:   inhibit,
		uninhibit: uninhibit,
		held:      make(map[Reason]bool),
	}
}

/**
 * Acquire holds reason, inhibiting the night light if no reason was held.
 */
func (i *Inhibitor) Acquire(reason Reason) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if len(i.held) > 0 {
		i.held[reason] = true
		return nil
	}
	if err := i.inhibitLocked(); err != nil {
		return err
	}
	i.held[reason] = true
	return nil
}

/**
 * Release drops reason, and uninhibits the night light once no reason is left.
 * A reason that was never acquired is a no-op. If the uninhibit fails, reason
 * stays held so a later release retries it.
 */
func (i *Inhibitor) Release(reason Reason) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if !i.held[reason] {
		return nil
	}
	if len(i.held) > 1 {
		delete(i.held, reason)
		return nil
	}
	if err := i.uninhibitLocked(); err != nil {
		return err
	}
	delete(i.held, reason)
	return nil
}

/**
 * ReleaseAll drops every held reason, for shutdown. On failure every reason
 * stays held.
 */
func (i *Inhibitor) ReleaseAll() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if len(i.held) == 0 {
		return nil
	}
	if err := i.uninhibitLocked(); err != nil {
		return err
	}
	clear(i.held)
	return nil
}

func (i *Inhibitor) inhibitLocked() error {
	ctx, cancel := context.WithTimeout(context.Background(), kwinCallTimeout)
	defer cancel()
	cookie, err := i.inhibit(ctx)
	if err != nil {
		return fmt.Errorf("inhibit night light: %w", err)
	}
	i.cookie = cookie
	return nil
}

func (i *Inhibitor) uninhibitLocked() error {
	ctx, cancel := context.WithTimeout(context.Background(), kwinCallTimeout)
	defer cancel()
	if err := i.uninhibit(ctx, i.cookie); err != nil {
		return fmt.Errorf("uninhibit night light: %w", err)
	}
	return nil
}
