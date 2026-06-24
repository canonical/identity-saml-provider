// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/service"
	"github.com/canonical/identity-saml-provider/internal/tracing"
	"github.com/canonical/identity-saml-provider/mocks"
	"go.uber.org/mock/gomock"
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
					SAMLAttributeMappings: map[string]domain.SAMLAttributeDef{
						"subject": {Name: "uid"},
						"email":   {Name: "mail"},
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
			logger := logging.NewNopLogger()

			tt.setupMock(mockRepo)

			svc := service.NewServiceProviderService(mockRepo, logger, tracing.NewNoopTracer())
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
			logger := logging.NewNopLogger()

			tt.setupMock(mockRepo)

			svc := service.NewServiceProviderService(mockRepo, logger, tracing.NewNoopTracer())
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

func TestServiceProviderService_UpdateAttributeMapping(t *testing.T) {
	t.Parallel()

	validMapping := &domain.AttributeMapping{
		NameIDFormat: "persistent",
		SAMLAttributeMappings: map[string]domain.SAMLAttributeDef{
			"email": {Name: "mail"},
		},
	}
	repoErr := errors.New("boom")

	tests := []struct {
		name      string
		entityID  string
		mapping   *domain.AttributeMapping
		setupMock func(repo *mocks.MockServiceProviderRepository)
		wantErr   bool
		errAs     interface{}
	}{
		{
			name:      "nil mapping is rejected with validation error",
			entityID:  "https://sp.example.com",
			mapping:   nil,
			setupMock: func(repo *mocks.MockServiceProviderRepository) {},
			wantErr:   true,
			errAs:     &domain.ErrValidation{},
		},
		{
			name:     "invalid nameid_format is rejected",
			entityID: "https://sp.example.com",
			mapping: &domain.AttributeMapping{
				NameIDFormat: "foobar",
			},
			setupMock: func(repo *mocks.MockServiceProviderRepository) {},
			wantErr:   true,
			errAs:     &domain.ErrValidation{},
		},
		{
			name:     "empty saml attribute name is rejected",
			entityID: "https://sp.example.com",
			mapping: &domain.AttributeMapping{
				SAMLAttributeMappings: map[string]domain.SAMLAttributeDef{
					"email": {Name: ""},
				},
			},
			setupMock: func(repo *mocks.MockServiceProviderRepository) {},
			wantErr:   true,
			errAs:     &domain.ErrValidation{},
		},
		{
			name:     "unknown SP returns ErrNotFound",
			entityID: "https://unknown.example.com",
			mapping:  validMapping,
			setupMock: func(repo *mocks.MockServiceProviderRepository) {
				repo.EXPECT().
					UpdateAttributeMapping(gomock.Any(), "https://unknown.example.com", validMapping).
					Return(&domain.ErrNotFound{Resource: "service_provider", ID: "https://unknown.example.com"})
			},
			wantErr: true,
			errAs:   &domain.ErrNotFound{},
		},
		{
			name:     "successful update calls repo once with the validated mapping",
			entityID: "https://sp.example.com",
			mapping:  validMapping,
			setupMock: func(repo *mocks.MockServiceProviderRepository) {
				repo.EXPECT().
					UpdateAttributeMapping(gomock.Any(), "https://sp.example.com", validMapping).
					Return(nil)
			},
		},
		{
			name:     "repository error is surfaced",
			entityID: "https://sp.example.com",
			mapping:  validMapping,
			setupMock: func(repo *mocks.MockServiceProviderRepository) {
				repo.EXPECT().
					UpdateAttributeMapping(gomock.Any(), "https://sp.example.com", validMapping).
					Return(repoErr)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
			tt.setupMock(mockRepo)

			svc := service.NewServiceProviderService(mockRepo, logging.NewNopLogger(), tracing.NewNoopTracer())
			err := svc.UpdateAttributeMapping(context.Background(), tt.entityID, tt.mapping)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errAs != nil {
					switch tt.errAs.(type) {
					case *domain.ErrValidation:
						var got *domain.ErrValidation
						if !errors.As(err, &got) {
							t.Errorf("expected *domain.ErrValidation, got %T", err)
						}
					case *domain.ErrNotFound:
						var got *domain.ErrNotFound
						if !errors.As(err, &got) {
							t.Errorf("expected *domain.ErrNotFound, got %T", err)
						}
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

func TestServiceProviderService_ClearAttributeMapping(t *testing.T) {
	t.Parallel()

	repoErr := errors.New("boom")

	tests := []struct {
		name      string
		entityID  string
		setupMock func(repo *mocks.MockServiceProviderRepository)
		wantErr   bool
		errAs     interface{}
	}{
		{
			name:     "unknown SP returns ErrNotFound",
			entityID: "https://unknown.example.com",
			setupMock: func(repo *mocks.MockServiceProviderRepository) {
				repo.EXPECT().
					UpdateAttributeMapping(gomock.Any(), "https://unknown.example.com", gomock.Nil()).
					Return(&domain.ErrNotFound{Resource: "service_provider", ID: "https://unknown.example.com"})
			},
			wantErr: true,
			errAs:   &domain.ErrNotFound{},
		},
		{
			name:     "successful clear calls repo once with nil",
			entityID: "https://sp.example.com",
			setupMock: func(repo *mocks.MockServiceProviderRepository) {
				repo.EXPECT().
					UpdateAttributeMapping(gomock.Any(), "https://sp.example.com", gomock.Nil()).
					Return(nil)
			},
		},
		{
			name:     "repository error is surfaced",
			entityID: "https://sp.example.com",
			setupMock: func(repo *mocks.MockServiceProviderRepository) {
				repo.EXPECT().
					UpdateAttributeMapping(gomock.Any(), "https://sp.example.com", gomock.Nil()).
					Return(repoErr)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
			tt.setupMock(mockRepo)

			svc := service.NewServiceProviderService(mockRepo, logging.NewNopLogger(), tracing.NewNoopTracer())
			err := svc.ClearAttributeMapping(context.Background(), tt.entityID)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errAs != nil {
					switch tt.errAs.(type) {
					case *domain.ErrNotFound:
						var got *domain.ErrNotFound
						if !errors.As(err, &got) {
							t.Errorf("expected *domain.ErrNotFound, got %T", err)
						}
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
