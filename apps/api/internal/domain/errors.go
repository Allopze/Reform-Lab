package domain

import "errors"

// Sentinel errors for classified failure modes.
var (
	ErrFormatUnsupported    = errors.New("format unsupported")
	ErrInvalidCorrupted     = errors.New("file is invalid or corrupted")
	ErrLimitExceeded        = errors.New("file exceeds size limit")
	ErrProtectedUnsupported = errors.New("protected or encrypted file not supported")
	ErrFileNotFound         = errors.New("file not found")
	ErrJobNotFound          = errors.New("job not found")
	ErrArtifactNotFound     = errors.New("artifact not found")
	ErrInvalidTransition    = errors.New("invalid job state transition")
	ErrCapabilityNotFound   = errors.New("capability not found")
	ErrCapabilityIneligible = errors.New("capability not eligible for this file")

	ErrEmailAlreadyExists = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserNotFound       = errors.New("user not found")
	ErrForbidden          = errors.New("forbidden")
	ErrArtifactExpired    = errors.New("artifact expired")
	ErrQuotaExceeded      = errors.New("quota exceeded")
	ErrTooManyActiveJobs  = errors.New("too many active jobs")
)
