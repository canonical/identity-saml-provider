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

func TestServiceProviderService_Register(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		sp        *domain.ServiceProvider
		setupMock func(repo *mocks.MockServiceProviderRepository)
		wantErr   bool
		errType   interface{}
	}{
		{
			name: "success without mapping",
			sp: &domain.ServiceProvider{
				EntityID:   "https://sp.example.com",
				ACSURL:     "https://sp.example.com/acs",
				ACSBinding: "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
			},
			setupMock: func(repo *mocks.MockServiceProviderRepository) {
				repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "success with valid mapping",
			sp: &domain.ServiceProvider{
				EntityID:   "https://sp.example.com",
				ACSURL:     "https://sp.example.com/acs",
				ACSBinding: "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
				AttributeMapping: &domain.AttributeMapping{
					NameIDFormat: "persistent",
					SAMLAttributes: map[string]string{
						"subject": "uid",
						"email":   "mail",
					},
				},
			},
			setupMock: func(repo *mocks.MockServiceProviderRepository) {
				repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "invalid mapping validation error",
			sp: &domain.ServiceProvider{
				EntityID:   "https://sp.example.com",
				ACSURL:     "https://sp.example.com/acs",
				ACSBinding: "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
				AttributeMapping: &domain.AttributeMapping{
					NameIDFormat: "INVALID_FORMAT",
				},
			},
			setupMock: func(repo *mocks.MockServiceProviderRepository) {
				// No repo call expected — validation fails first
			},
			wantErr: true,
			errType: &domain.ErrValidation{},
		},
		{
			name: "conflict error - already exists",
			sp: &domain.ServiceProvider{
				EntityID:   "https://sp.example.com",
				ACSURL:     "https://sp.example.com/acs",
				ACSBinding: "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
			},
			setupMock: func(repo *mocks.MockServiceProviderRepository) {
				repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(&domain.ErrConflict{Resource: "service_provider", ID: "https://sp.example.com"})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
			logger := zap.NewNop().Sugar()

			tt.setupMock(mockRepo)

			svc := service.NewServiceProviderService(mockRepo, logger)
			err := svc.Register(context.Background(), tt.sp)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errType != nil {
					var validationErr *domain.ErrValidation
					if !errors.As(err, &validationErr) {
						t.Errorf("expected *domain.ErrValidation, got %T: %v", err, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestServiceProviderService_GetByEntityID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		entityID  string
		setupMock func(repo *mocks.MockServiceProviderRepository)
		wantErr   bool
		errType   interface{}
	}{
		{
			name:     "success",
			entityID: "https://sp.example.com",
			setupMock: func(repo *mocks.MockServiceProviderRepository) {
				repo.EXPECT().GetByEntityID(gomock.Any(), "https://sp.example.com").Return(&domain.ServiceProvider{
					EntityID: "https://sp.example.com",
					ACSURL:   "https://sp.example.com/acs",
				}, nil)
			},
		},
		{
			name:     "not found",
			entityID: "https://unknown.example.com",
			setupMock: func(repo *mocks.MockServiceProviderRepository) {
				repo.EXPECT().GetByEntityID(gomock.Any(), "https://unknown.example.com").Return(nil, &domain.ErrNotFound{Resource: "service_provider", ID: "https://unknown.example.com"})
			},
			wantErr: true,
			errType: &domain.ErrNotFound{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
			logger := zap.NewNop().Sugar()

			tt.setupMock(mockRepo)

			svc := service.NewServiceProviderService(mockRepo, logger)
			result, err := svc.GetByEntityID(context.Background(), tt.entityID)

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
			if result.EntityID != tt.entityID {
				t.Errorf("EntityID = %q, want %q", result.EntityID, tt.entityID)
			}
		})
	}
}
