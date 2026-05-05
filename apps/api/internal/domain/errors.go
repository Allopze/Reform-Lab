package domain

import "errors"

// Sentinel errors for classified failure modes.
var (
	ErrFormatUnsupported      = errors.New("format unsupported")
	ErrInvalidCorrupted       = errors.New("file is invalid or corrupted")
	ErrLimitExceeded          = errors.New("file exceeds size limit")
	ErrImageDimensionsUnknown = errors.New("image dimensions could not be verified")
	ErrProtectedUnsupported   = errors.New("protected or encrypted file not supported")
	ErrFileNotFound           = errors.New("file not found")
	ErrJobNotFound            = errors.New("job not found")
	ErrArtifactNotFound       = errors.New("artifact not found")
	ErrInvalidTransition      = errors.New("invalid job state transition")
	ErrCapabilityNotFound     = errors.New("capability not found")
	ErrCapabilityIneligible   = errors.New("capability not eligible for this file")

	ErrEmailAlreadyExists            = errors.New("email already registered")
	ErrInvalidCredentials            = errors.New("invalid email or password")
	ErrUserNotFound                  = errors.New("user not found")
	ErrBootstrapAdminRequired        = errors.New("initial admin bootstrap required")
	ErrUserSuspended                 = errors.New("user suspended")
	ErrSessionRevoked                = errors.New("session revoked")
	ErrPasswordResetTokenInvalid     = errors.New("password reset token invalid")
	ErrEmailVerificationTokenInvalid = errors.New("email verification token invalid")
	ErrJobIntakePaused               = errors.New("job intake paused")
	ErrForbidden                     = errors.New("forbidden")
	ErrArtifactExpired               = errors.New("artifact expired")
	ErrQuotaExceeded                 = errors.New("quota exceeded")
	ErrTooManyActiveJobs             = errors.New("too many active jobs")
)
