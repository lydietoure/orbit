package db

import (
	"context"
	"errors"
	"testing"
)

func TestGetDockRoot_FreshDBIsUnset(t *testing.T) {
	db := newTestDB(t)

	_, err := GetDockRoot(context.Background(), db)
	if !errors.Is(err, ErrNoDockRoot) {
		t.Errorf("fresh DB GetDockRoot err = %v, want ErrNoDockRoot", err)
	}
}

func TestSetDockRoot_PersistsAndGetReturnsIt(t *testing.T) {
	db := newTestDB(t)
	want := "/home/me/code/dock"

	if err := SetDockRoot(context.Background(), db, want); err != nil {
		t.Fatalf("SetDockRoot: %v", err)
	}

	got, err := GetDockRoot(context.Background(), db)
	if err != nil {
		t.Fatalf("GetDockRoot: %v", err)
	}
	if got != want {
		t.Errorf("GetDockRoot = %q, want %q", got, want)
	}
}

func TestSetDockRoot_Overwrites(t *testing.T) {
	db := newTestDB(t)
	if err := SetDockRoot(context.Background(), db, "/first"); err != nil {
		t.Fatalf("SetDockRoot first: %v", err)
	}
	if err := SetDockRoot(context.Background(), db, "/second"); err != nil {
		t.Fatalf("SetDockRoot second: %v", err)
	}

	got, err := GetDockRoot(context.Background(), db)
	if err != nil {
		t.Fatalf("GetDockRoot: %v", err)
	}
	if got != "/second" {
		t.Errorf("GetDockRoot = %q, want /second", got)
	}
}

func TestSetDockRoot_EmptyStringUnsets(t *testing.T) {
	db := newTestDB(t)
	if err := SetDockRoot(context.Background(), db, "/something"); err != nil {
		t.Fatalf("SetDockRoot: %v", err)
	}

	// Passing "" routes through UnsetDockRoot — explicitly documented
	// behavior so callers can't accidentally store an empty path
	// that reads back as "configured but blank".
	if err := SetDockRoot(context.Background(), db, ""); err != nil {
		t.Fatalf("SetDockRoot empty: %v", err)
	}

	_, err := GetDockRoot(context.Background(), db)
	if !errors.Is(err, ErrNoDockRoot) {
		t.Errorf("after Set(\"\"), Get err = %v, want ErrNoDockRoot", err)
	}
}

func TestUnsetDockRoot_ClearsConfiguredValue(t *testing.T) {
	db := newTestDB(t)
	if err := SetDockRoot(context.Background(), db, "/configured"); err != nil {
		t.Fatalf("SetDockRoot: %v", err)
	}

	if err := UnsetDockRoot(context.Background(), db); err != nil {
		t.Fatalf("UnsetDockRoot: %v", err)
	}

	_, err := GetDockRoot(context.Background(), db)
	if !errors.Is(err, ErrNoDockRoot) {
		t.Errorf("after Unset, Get err = %v, want ErrNoDockRoot", err)
	}
}

func TestUnsetDockRoot_NoOpWhenUnset(t *testing.T) {
	db := newTestDB(t)
	// Fresh DB has no dock root; Unset should silently succeed.
	if err := UnsetDockRoot(context.Background(), db); err != nil {
		t.Errorf("UnsetDockRoot on fresh DB: %v", err)
	}
}

func TestGetDockAutoCreate_DefaultsFalse(t *testing.T) {
	db := newTestDB(t)

	got, err := GetDockAutoCreate(context.Background(), db)
	if err != nil {
		t.Fatalf("GetDockAutoCreate: %v", err)
	}
	if got {
		t.Error("fresh DB dock_auto_create = true, want false")
	}
}

func TestSetDockAutoCreate_RoundTrip(t *testing.T) {
	db := newTestDB(t)

	if err := SetDockAutoCreate(context.Background(), db, true); err != nil {
		t.Fatalf("SetDockAutoCreate(true): %v", err)
	}
	got, err := GetDockAutoCreate(context.Background(), db)
	if err != nil {
		t.Fatalf("GetDockAutoCreate: %v", err)
	}
	if !got {
		t.Error("after Set(true), Get = false")
	}

	if err := SetDockAutoCreate(context.Background(), db, false); err != nil {
		t.Fatalf("SetDockAutoCreate(false): %v", err)
	}
	got, err = GetDockAutoCreate(context.Background(), db)
	if err != nil {
		t.Fatalf("GetDockAutoCreate: %v", err)
	}
	if got {
		t.Error("after Set(false), Get = true")
	}
}
