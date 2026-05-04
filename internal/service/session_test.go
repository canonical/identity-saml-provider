package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/service"
	"github.com/canonical/identity-saml-provider/mocks"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestSessionService_CreateFromOIDC(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		claims      *service.OIDCClaims
		setupMock   func(repo *mocks.MockSessionRepository)
		wantErr     bool
		errType     interface{}
		checkResult func(t *testing.T, s *domain.Session)
	}{
		{
			name: "success with all claims",
			claims: &service.OIDCClaims{
				Sub:    "user-sub-123",
				Email:  "user@example.com",
				Name:   "Jane Doe",
				Groups: []string{"admin", "users"},
				RawClaims: map[string]interface{}{
					"sub":   "user-sub-123",
					"email": "user@example.com",
					"name":  "Jane Doe",
				},
			},
			setupMock: func(repo *mocks.MockSessionRepository) {
				repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
			},
			checkResult: func(t *testing.T, s *domain.Session) {
				t.Helper()
				if s.UserEmail != "user@example.com" {
					t.Errorf("UserEmail = %q, want %q", s.UserEmail, "user@example.com")
				}
				if s.UserCommonName != "Jane Doe" {
					t.Errorf("UserCommonName = %q, want %q", s.UserCommonName, "Jane Doe")
				}
				if s.UserName != "user-sub-123" {
					t.Errorf("UserName = %q, want %q", s.UserName, "user-sub-123")
				}
				if s.NameID != "user@example.com" {
					t.Errorf("NameID = %q, want %q", s.NameID, "user@example.com")
				}
				if len(s.Groups) != 2 {
					t.Errorf("Groups len = %d, want 2", len(s.Groups))
				}
				if s.RawOIDCClaims == nil {
					t.Error("RawOIDCClaims should not be nil")
				}
				if s.ID == "" {
					t.Error("ID should not be empty")
				}
			},
		},
		{
			name: "success - name falls back to email when name is empty",
			claims: &service.OIDCClaims{
				Sub:   "user-sub-123",
				Email: "user@example.com",
			},
			setupMock: func(repo *mocks.MockSessionRepository) {
				repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
			},
			checkResult: func(t *testing.T, s *domain.Session) {
				t.Helper()
				if s.UserCommonName != "user@example.com" {
					t.Errorf("UserCommonName = %q, want %q (fallback to email)", s.UserCommonName, "user@example.com")
				}
			},
		},
		{
			name: "missing email returns validation error",
			claims: &service.OIDCClaims{
				Sub:  "user-sub-123",
				Name: "Jane Doe",
			},
			setupMock: func(repo *mocks.MockSessionRepository) {
				// No repo call expected
			},
			wantErr: true,
			errType: &domain.ErrValidation{},
		},
		{
			name: "repo save error",
			claims: &service.OIDCClaims{
				Sub:   "user-sub-123",
				Email: "user@example.com",
			},
			setupMock: func(repo *mocks.MockSessionRepository) {
				repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(errors.New("db connection lost"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			mockRepo := mocks.NewMockSessionRepository(ctrl)
			logger := zap.NewNop().Sugar()

			tt.setupMock(mockRepo)

			svc := service.NewSessionService(mockRepo, logger)
			result, err := svc.CreateFromOIDC(context.Background(), tt.claims)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errType != nil {
					var validationErr *domain.ErrValidation
					if !errors.As(err, &validationErr) {
						t.Errorf("expected *domain.ErrValidation, got %T", err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}

func TestSessionService_GetByID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		id        string
		setupMock func(repo *mocks.MockSessionRepository)
		wantErr   bool
		errType   interface{}
	}{
		{
			name: "success",
			id:   "session-123",
			setupMock: func(repo *mocks.MockSessionRepository) {
				repo.EXPECT().GetByID(gomock.Any(), "session-123").Return(&domain.Session{
					ID:        "session-123",
					UserEmail: "user@example.com",
				}, nil)
			},
		},
		{
			name: "not found",
			id:   "missing-id",
			setupMock: func(repo *mocks.MockSessionRepository) {
				repo.EXPECT().GetByID(gomock.Any(), "missing-id").Return(nil, &domain.ErrNotFound{Resource: "session", ID: "missing-id"})
			},
			wantErr: true,
			errType: &domain.ErrNotFound{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			mockRepo := mocks.NewMockSessionRepository(ctrl)
			logger := zap.NewNop().Sugar()

			tt.setupMock(mockRepo)

			svc := service.NewSessionService(mockRepo, logger)
			result, err := svc.GetByID(context.Background(), tt.id)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errType != nil {
					var notFoundErr *domain.ErrNotFound
					if !errors.As(err, &notFoundErr) {
						t.Errorf("expected *domain.ErrNotFound, got %T", err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.ID != tt.id {
				t.Errorf("result.ID = %q, want %q", result.ID, tt.id)
			}
		})
	}
}

func TestSessionService_CleanupExpired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupMock func(repo *mocks.MockSessionRepository)
		wantCount int64
		wantErr   bool
	}{
		{
			name: "cleanup success with deletions",
			setupMock: func(repo *mocks.MockSessionRepository) {
				repo.EXPECT().DeleteExpired(gomock.Any()).Return(int64(5), nil)
			},
			wantCount: 5,
		},
		{
			name: "cleanup success with no deletions",
			setupMock: func(repo *mocks.MockSessionRepository) {
				repo.EXPECT().DeleteExpired(gomock.Any()).Return(int64(0), nil)
			},
			wantCount: 0,
		},
		{
			name: "cleanup error",
			setupMock: func(repo *mocks.MockSessionRepository) {
				repo.EXPECT().DeleteExpired(gomock.Any()).Return(int64(0), errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			mockRepo := mocks.NewMockSessionRepository(ctrl)
			logger := zap.NewNop().Sugar()

			tt.setupMock(mockRepo)

			svc := service.NewSessionService(mockRepo, logger)
			count, err := svc.CleanupExpired(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if count != tt.wantCount {
				t.Errorf("count = %d, want %d", count, tt.wantCount)
			}
		})
	}
}
