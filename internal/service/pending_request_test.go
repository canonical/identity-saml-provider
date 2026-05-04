package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/service"
	"github.com/canonical/identity-saml-provider/mocks"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestPendingRequestService_Store(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		req       *domain.PendingAuthnRequest
		setupMock func(repo *mocks.MockPendingRequestRepository)
		wantErr   bool
	}{
		{
			name: "success",
			req: &domain.PendingAuthnRequest{
				RequestID:   "req-123",
				SAMLRequest: "<saml>request</saml>",
				RelayState:  "relay-state-456",
				CreatedAt:   time.Now(),
			},
			setupMock: func(repo *mocks.MockPendingRequestRepository) {
				repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "repo error",
			req: &domain.PendingAuthnRequest{
				RequestID: "req-123",
			},
			setupMock: func(repo *mocks.MockPendingRequestRepository) {
				repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(errors.New("storage error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			mockRepo := mocks.NewMockPendingRequestRepository(ctrl)
			logger := zap.NewNop().Sugar()

			tt.setupMock(mockRepo)

			svc := service.NewPendingRequestService(mockRepo, logger)
			err := svc.Store(context.Background(), tt.req)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestPendingRequestService_Retrieve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		requestID string
		setupMock func(repo *mocks.MockPendingRequestRepository)
		wantErr   bool
		errType   interface{}
	}{
		{
			name:      "success",
			requestID: "req-123",
			setupMock: func(repo *mocks.MockPendingRequestRepository) {
				repo.EXPECT().GetAndDelete(gomock.Any(), "req-123").Return(&domain.PendingAuthnRequest{
					RequestID:   "req-123",
					SAMLRequest: "<saml>request</saml>",
					RelayState:  "relay-state",
					CreatedAt:   time.Now(),
				}, nil)
			},
		},
		{
			name:      "not found",
			requestID: "missing-req",
			setupMock: func(repo *mocks.MockPendingRequestRepository) {
				repo.EXPECT().GetAndDelete(gomock.Any(), "missing-req").Return(nil, &domain.ErrNotFound{Resource: "pending_request", ID: "missing-req"})
			},
			wantErr: true,
			errType: &domain.ErrNotFound{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			mockRepo := mocks.NewMockPendingRequestRepository(ctrl)
			logger := zap.NewNop().Sugar()

			tt.setupMock(mockRepo)

			svc := service.NewPendingRequestService(mockRepo, logger)
			result, err := svc.Retrieve(context.Background(), tt.requestID)

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
			if result.RequestID != tt.requestID {
				t.Errorf("RequestID = %q, want %q", result.RequestID, tt.requestID)
			}
		})
	}
}
