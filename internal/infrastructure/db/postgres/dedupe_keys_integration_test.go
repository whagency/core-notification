package postgres_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"notification-service/internal/infrastructure/db/postgres"

	"github.com/whagency/core-common/testkit/loggertest"
	"github.com/whagency/core-common/testkit/postgrestest"
	"gorm.io/gorm"
)

var suite *postgrestest.Suite

func TestMain(m *testing.M) {
	var err error
	suite, err = postgrestest.Start(context.Background(), postgrestest.ContainerConfig{})
	if err != nil {
		panic("postgrestest: start: " + err.Error())
	}
	code := m.Run()
	_ = suite.Terminate(context.Background())
	os.Exit(code)
}

func openDB(t *testing.T) *gorm.DB {
	t.Helper()
	loggertest.Init(t, "notification-service")

	return suite.OpenGormDB(t, postgrestest.OpenConfig{
		MigrationsDir:     migrationsDir(t),
		SkipMigrationLine: postgrestest.SkipDatabaseLevelMigrationLine,
	})
}

// migrationsDir resolves the real migration directory relative to this file so
// the tests exercise the schema the service actually ships.
func migrationsDir(t testing.TB) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed while resolving postgres migrations directory")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../data/db/migrations/postgres"))
}

func TestDedupeKeyRepo_ReserveIsIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := postgres.NewDedupeKeyRepository(openDB(t))

	already, err := repo.Reserve(ctx, "push:transfer:1:outgoing", time.Hour)
	if err != nil {
		t.Fatalf("first reserve: %v", err)
	}
	if already {
		t.Error("first reserve must claim the fence")
	}

	already, err = repo.Reserve(ctx, "push:transfer:1:outgoing", time.Hour)
	if err != nil {
		t.Fatalf("second reserve: %v", err)
	}
	if !already {
		t.Error("second reserve must report the fence as already claimed")
	}
}

// The fence is the whole guarantee, so it has to hold when several consumers
// process the same republished message at once.
func TestDedupeKeyRepo_ConcurrentReserveAdmitsExactlyOne(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := postgres.NewDedupeKeyRepository(openDB(t))

	const racers = 8
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		winners int
	)

	for range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			already, err := repo.Reserve(ctx, "push:contended", time.Hour)
			if err != nil {
				t.Errorf("reserve: %v", err)
				return
			}
			if !already {
				mu.Lock()
				winners++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if winners != 1 {
		t.Errorf("consumers admitted = %d, want exactly 1", winners)
	}
}

func TestDedupeKeyRepo_ReleaseAllowsReReservation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := postgres.NewDedupeKeyRepository(openDB(t))

	if _, err := repo.Reserve(ctx, "push:released", time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := repo.Release(ctx, "push:released"); err != nil {
		t.Fatalf("release: %v", err)
	}

	already, err := repo.Reserve(ctx, "push:released", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if already {
		t.Error("a released fence must be claimable again")
	}
}

// Release of an unknown key happens whenever a send fails without a fence; it
// must not surface as an error and abort the caller's failure path.
func TestDedupeKeyRepo_ReleaseUnknownKeyIsNoOp(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := postgres.NewDedupeKeyRepository(openDB(t))

	if err := repo.Release(ctx, "never-reserved"); err != nil {
		t.Errorf("release of unknown key: %v", err)
	}
}

func TestDedupeKeyRepo_DeleteExpiredRemovesOnlyLapsedFences(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := postgres.NewDedupeKeyRepository(openDB(t))

	// A negative TTL lands expires_at in the past, standing in for an old fence.
	if _, err := repo.Reserve(ctx, "push:expired", -time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Reserve(ctx, "push:live", time.Hour); err != nil {
		t.Fatal(err)
	}

	deleted, err := repo.DeleteExpired(ctx)
	if err != nil {
		t.Fatalf("delete expired: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	already, err := repo.Reserve(ctx, "push:live", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !already {
		t.Error("the unexpired fence must survive cleanup")
	}
}
