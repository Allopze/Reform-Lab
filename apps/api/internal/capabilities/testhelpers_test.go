package capabilities

import (
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/google/uuid"
)

// Test helpers for capabilities package.

func fakePDFFile() domain.OriginalFile {
	return domain.OriginalFile{
		ID:           uuid.New(),
		InternalName: "test.pdf",
		OriginalName: "test.pdf",
		Size:         1024,
		DetectedFormat: domain.DetectedFormat{
			MIMEType:  "application/pdf",
			Family:    domain.FamilyPDF,
			Extension: "pdf",
		},
		Metadata:   domain.FileMetadata{},
		UploadedAt: time.Now(),
	}
}

func fakeImageFile(mime, ext string) domain.OriginalFile {
	return domain.OriginalFile{
		ID:           uuid.New(),
		InternalName: "test." + ext,
		OriginalName: "test." + ext,
		Size:         512,
		DetectedFormat: domain.DetectedFormat{
			MIMEType:  mime,
			Family:    domain.FamilyImage,
			Extension: ext,
		},
		Metadata:   domain.FileMetadata{},
		UploadedAt: time.Now(),
	}
}

func fakeTextFile(mime, ext string) domain.OriginalFile {
	return domain.OriginalFile{
		ID:           uuid.New(),
		InternalName: "test." + ext,
		OriginalName: "test." + ext,
		Size:         512,
		DetectedFormat: domain.DetectedFormat{
			MIMEType:  mime,
			Family:    domain.FamilyDocument,
			Extension: ext,
		},
		Metadata:   domain.FileMetadata{},
		UploadedAt: time.Now(),
	}
}

func fakeDocumentFile(mime, ext string) domain.OriginalFile {
	return domain.OriginalFile{
		ID:           uuid.New(),
		InternalName: "test." + ext,
		OriginalName: "test." + ext,
		Size:         1024,
		DetectedFormat: domain.DetectedFormat{
			MIMEType:  mime,
			Family:    domain.FamilyDocument,
			Extension: ext,
		},
		Metadata:   domain.FileMetadata{},
		UploadedAt: time.Now(),
	}
}

func fakePresentationFile(mime, ext string) domain.OriginalFile {
	return domain.OriginalFile{
		ID:           uuid.New(),
		InternalName: "deck." + ext,
		OriginalName: "deck." + ext,
		Size:         2048,
		DetectedFormat: domain.DetectedFormat{
			MIMEType:  mime,
			Family:    domain.FamilyDocument,
			Extension: ext,
		},
		Metadata:   domain.FileMetadata{},
		UploadedAt: time.Now(),
	}
}

func fakeSpreadsheetFile(mime, ext string) domain.OriginalFile {
	return domain.OriginalFile{
		ID:           uuid.New(),
		InternalName: "sheet." + ext,
		OriginalName: "sheet." + ext,
		Size:         1024,
		DetectedFormat: domain.DetectedFormat{
			MIMEType:  mime,
			Family:    domain.FamilyDocument,
			Extension: ext,
		},
		Metadata:   domain.FileMetadata{},
		UploadedAt: time.Now(),
	}
}

func fakeVideoFile(mime, ext string) domain.OriginalFile {
	return domain.OriginalFile{
		ID:           uuid.New(),
		InternalName: "test." + ext,
		OriginalName: "test." + ext,
		Size:         2048,
		DetectedFormat: domain.DetectedFormat{
			MIMEType:  mime,
			Family:    domain.FamilyVideo,
			Extension: ext,
		},
		Metadata:   domain.FileMetadata{},
		UploadedAt: time.Now(),
	}
}

func fakeAudioFile(mime, ext string) domain.OriginalFile {
	return domain.OriginalFile{
		ID:           uuid.New(),
		InternalName: "test." + ext,
		OriginalName: "test." + ext,
		Size:         1024,
		DetectedFormat: domain.DetectedFormat{
			MIMEType:  mime,
			Family:    domain.FamilyAudio,
			Extension: ext,
		},
		Metadata:   domain.FileMetadata{},
		UploadedAt: time.Now(),
	}
}
