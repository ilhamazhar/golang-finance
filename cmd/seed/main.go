// Command seed populates the users table with synthetic accounts for local
// development and load testing. It reuses the production domain model and
// password hasher so seeded rows are indistinguishable from real ones.
//
// Usage:
//
//	go run ./cmd/seed                 # 1000 users, default password
//	go run ./cmd/seed -count 5000     # custom volume
//	go run ./cmd/seed -password secret -seed 7
//
// Requires DATABASE_URL (read from the environment or a .env file). Every
// seeded account shares one password and uses the @seed.azhar.test domain so
// they are easy to spot and bulk-delete.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/ilhamazhar/golang-gpt/internal/domain"
	"github.com/ilhamazhar/golang-gpt/pkg/password"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// seedDomain marks every generated email so seed data is trivially identifiable
// and removable: DELETE FROM users WHERE email LIKE '%@seed.azhar.test';
const seedDomain = "seed.azhar.test"

var firstNames = []string{
	"Budi", "Siti", "Agus", "Dewi", "Eko", "Rina", "Andi", "Putri", "Joko", "Ayu",
	"Rizki", "Nur", "Bayu", "Indah", "Fajar", "Maya", "Dian", "Hadi", "Lestari", "Yusuf",
	"Sari", "Wahyu", "Ratna", "Iqbal", "Fitri", "Adi", "Citra", "Galih", "Wulan", "Surya",
}

var lastNames = []string{
	"Santoso", "Wijaya", "Pratama", "Nugroho", "Saputra", "Hidayat", "Kurniawan", "Lestari",
	"Halim", "Setiawan", "Utomo", "Permata", "Anggraini", "Maulana", "Rahman", "Suryanto",
	"Hartono", "Gunawan", "Cahyono", "Puspita", "Firmansyah", "Wibowo", "Mahendra", "Susanto",
}

// roleFor distributes roles realistically: a handful of admins, a back-office
// layer of staff, and everyone else as ordinary users.
func roleFor(i int) domain.Role {
	switch {
	case i%500 == 0:
		return domain.RoleAdmin
	case i%25 == 0:
		return domain.RoleStaff
	default:
		return domain.RoleUser
	}
}

func main() {
	count := flag.Int("count", 1000, "number of users to create")
	pass := flag.String("password", "Password123!", "plaintext password shared by all seeded users")
	seed := flag.Int64("seed", 1, "RNG seed for reproducible data")
	batch := flag.Int("batch", 200, "insert batch size")
	flag.Parse()

	if *count < 1 {
		log.Fatal("count must be a positive integer")
	}

	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using environment variables")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		// Quiet the per-statement logger; batch inserts are noisy at scale.
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	if err := db.AutoMigrate(&domain.User{}); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	// Hash the shared password once — Argon2id is deliberately expensive, so
	// hashing per row would dominate runtime for no benefit on seed data.
	hash, err := password.Hash(*pass, password.DefaultParams)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	rng := rand.New(rand.NewSource(*seed))
	now := time.Now()
	users := make([]domain.User, 0, *count)

	for i := 0; i < *count; i++ {
		first := firstNames[rng.Intn(len(firstNames))]
		last := lastNames[rng.Intn(len(lastNames))]

		// Index-suffixed local part guarantees uniqueness even when the random
		// name pair repeats across 1000+ rows.
		email := fmt.Sprintf("%s.%s%d@%s", first, last, i, seedDomain)

		// Spread CreatedAt across the past year so list/sort/pagination views
		// have realistic ordering instead of one identical timestamp.
		created := now.Add(-time.Duration(rng.Intn(365*24)) * time.Hour)

		u := domain.User{
			ID:           uuid.New(),
			Name:         first + " " + last,
			Email:        email,
			PasswordHash: hash,
			Role:         roleFor(i),
			CreatedAt:    created,
			UpdatedAt:    created,
		}

		// ~70% of accounts have a verified email, at a time after sign-up.
		if rng.Intn(10) < 7 {
			verified := created.Add(time.Duration(rng.Intn(48)) * time.Hour)
			u.EmailVerifiedAt = &verified
		}

		users = append(users, u)
	}

	ctx := context.Background()
	if err := db.WithContext(ctx).CreateInBatches(users, *batch).Error; err != nil {
		log.Fatalf("insert users: %v", err)
	}

	log.Printf("seeded %d users (password %q, domain @%s)", *count, *pass, seedDomain)
}
