package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/akuaruu/url-shortener/shortener-service/internal/repository"
	"github.com/akuaruu/url-shortener/shortener-service/internal/service"
)

// MockRepository
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) CreateURL(ctx context.Context, originalURL string, expiresAt *time.Time) (*repository.URLRecord, error) {
	args := m.Called(ctx, originalURL, expiresAt)
	if args.Get(0) != nil {
		return args.Get(0).(*repository.URLRecord), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRepository) GetByShortCode(ctx context.Context, code string) (*repository.URLRecord, error) {
	args := m.Called(ctx, code)
	if args.Get(0) != nil {
		return args.Get(0).(*repository.URLRecord), args.Error(1)
	}
	return nil, args.Error(1)
}

// Test Case 1: Membuat URL Pendek Berhasil
func TestCreateShortURL_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := service.NewShortenerService(mockRepo, "https://aruu.app")

	originalURL := "https://go.dev/doc/effective_go"

	// Kita program mock-nya: JIKA dipanggil dengan parameter ini, MAKA kembalikan data palsu ini.
	mockRepo.On("CreateURL", mock.Anything, originalURL, mock.Anything).Return(&repository.URLRecord{
		ID:          1,
		ShortCode:   "1",
		OriginalURL: originalURL,
		CreatedAt:   time.Now(),
	}, nil)

	// Eksekusi fungsi yang mau dites
	result, err := svc.CreateShortURL(context.Background(), originalURL, 0)

	// Validasi hasil
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "https://aruu.app/1", result.ShortURL) // Memastikan baseURL dirangkai dengan benar
	assert.Equal(t, "1", result.ShortCode)
	mockRepo.AssertExpectations(t)
}

// Test Case 2: Format URL Invalid
func TestCreateShortURL_InvalidURL(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := service.NewShortenerService(mockRepo, "https://aruu.app")

	// Eksekusi dengan URL yang ngawur
	result, err := svc.CreateShortURL(context.Background(), "bukan-url-yang-valid", 0)

	// Validasi bahwa service harus menolak dan mengembalikan error
	assert.ErrorIs(t, err, service.ErrInvalidURL)
	assert.Nil(t, result)
}

// --- UNHAPPY PATHS: CreateShortURL ---

// Test Case 3: TTL Negatif (Edge Case Validasi)
func TestCreateShortURL_NegativeTTL(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := service.NewShortenerService(mockRepo, "https://aruu.app")

	result, err := svc.CreateShortURL(context.Background(), "https://go.dev", -1)

	assert.ErrorIs(t, err, service.ErrInvalidURL)
	assert.Contains(t, err.Error(), "ttl_seconds must be >= 0")
	assert.Nil(t, result)

	// Validasi Penting: Pastikan database TIDAK PERNAH dipanggil jika validasi gagal
	mockRepo.AssertNotCalled(t, "CreateURL")
}

// Test Case 4: Database Error / Timeout (Failure Mode)
func TestCreateShortURL_RepositoryError(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := service.NewShortenerService(mockRepo, "https://aruu.app")

	originalURL := "https://go.dev"
	dbError := errors.New("simulated database connection lost")

	// Kita paksa mock untuk mengembalikan error seolah-olah PostgreSQL mati
	mockRepo.On("CreateURL", mock.Anything, originalURL, mock.Anything).Return((*repository.URLRecord)(nil), dbError)

	result, err := svc.CreateShortURL(context.Background(), originalURL, 0)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create short url")
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// --- PENGUJIAN FUNGSI GET URL DETAILS ---

// Test Case 5: GetURLDetails - Success (Happy Path)
func TestGetURLDetails_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := service.NewShortenerService(mockRepo, "https://aruu.app")

	expectedRecord := &repository.URLRecord{
		OriginalURL: "https://go.dev",
		CreatedAt:   time.Now(),
		ClickCount:  42,
	}

	mockRepo.On("GetByShortCode", mock.Anything, "aZ3x").Return(expectedRecord, nil)

	result, err := svc.GetURLDetails(context.Background(), "aZ3x")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(42), result.ClickCount) // Memastikan metadata terbawa
	mockRepo.AssertExpectations(t)
}

// Test Case 6: Short Code Tidak Ditemukan (Failure Mode)
func TestGetURLDetails_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := service.NewShortenerService(mockRepo, "https://aruu.app")

	// Simulasi DB mengembalikan ErrNotFound bawaan repository
	mockRepo.On("GetByShortCode", mock.Anything, "invalidCode").Return((*repository.URLRecord)(nil), repository.ErrNotFound)

	result, err := svc.GetURLDetails(context.Background(), "invalidCode")

	// Service harus menerjemahkan error tersebut menjadi service.ErrNotFound
	assert.ErrorIs(t, err, service.ErrNotFound)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// Test Case 7: GetURLDetails - Database Error (Failure Mode)
func TestGetURLDetails_RepositoryError(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := service.NewShortenerService(mockRepo, "https://aruu.app")

	dbError := errors.New("simulated query timeout")
	mockRepo.On("GetByShortCode", mock.Anything, "aZ3x").Return((*repository.URLRecord)(nil), dbError)

	result, err := svc.GetURLDetails(context.Background(), "aZ3x")

	assert.Error(t, err)
	assert.NotErrorIs(t, err, service.ErrNotFound) // Pastikan ini BUKAN error not found, melainkan error sistem
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}
