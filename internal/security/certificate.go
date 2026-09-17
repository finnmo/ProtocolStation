package security

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
)

// CertificateInfo contains certificate expiration information
type CertificateInfo struct {
	NotBefore  time.Time
	NotAfter   time.Time
	Issuer     string
	Subject    string
	DaysUntil  int
	IsExpired  bool
	IsExpiring bool
}

// CheckCertificateExpiration checks the expiration status of a certificate file
func CheckCertificateExpiration(certPath string, logger *zap.Logger) (*CertificateInfo, error) {
	if certPath == "" {
		return nil, fmt.Errorf("certificate path is empty")
	}

	// Read certificate file
	data, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file: %w", err)
	}

	// Parse PEM block
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Parse certificate
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	now := time.Now()
	daysUntil := int(cert.NotAfter.Sub(now).Hours() / 24)
	isExpired := now.After(cert.NotAfter)
	isExpiring := daysUntil <= 30 && !isExpired

	info := &CertificateInfo{
		NotBefore:  cert.NotBefore,
		NotAfter:   cert.NotAfter,
		Issuer:     cert.Issuer.String(),
		Subject:    cert.Subject.String(),
		DaysUntil:  daysUntil,
		IsExpired:  isExpired,
		IsExpiring: isExpiring,
	}

	// Log warnings based on expiration status
	if isExpired {
		logger.Error("certificate has expired",
			zap.String("cert_path", certPath),
			zap.String("issuer", info.Issuer),
			zap.Time("expired_at", cert.NotAfter))
	} else if isExpiring {
		if daysUntil <= 1 {
			logger.Error("certificate expires in 1 day or less",
				zap.String("cert_path", certPath),
				zap.String("issuer", info.Issuer),
				zap.Int("days_remaining", daysUntil),
				zap.Time("expires_at", cert.NotAfter))
		} else if daysUntil <= 7 {
			logger.Warn("certificate expires within 7 days",
				zap.String("cert_path", certPath),
				zap.String("issuer", info.Issuer),
				zap.Int("days_remaining", daysUntil),
				zap.Time("expires_at", cert.NotAfter))
		} else {
			logger.Warn("certificate expires within 30 days",
				zap.String("cert_path", certPath),
				zap.String("issuer", info.Issuer),
				zap.Int("days_remaining", daysUntil),
				zap.Time("expires_at", cert.NotAfter))
		}
	}

	return info, nil
}

// ValidateTLSConfig validates a TLS configuration and checks certificate expiration
func ValidateTLSConfig(tlsConfig *tls.Config, certPath string, logger *zap.Logger) error {
	if certPath == "" {
		return nil
	}

	info, err := CheckCertificateExpiration(certPath, logger)
	if err != nil {
		logger.Warn("failed to check certificate expiration",
			zap.String("cert_path", certPath),
			zap.Error(err))
		return nil // Don't fail the connection, just log the warning
	}

	if info.IsExpired {
		return fmt.Errorf("certificate has expired: %s", certPath)
	}

	return nil
}

// CheckCertificateExpirationDays returns days until certificate expiration
func CheckCertificateExpirationDays(certPath string) (int, error) {
	if certPath == "" {
		return -1, fmt.Errorf("certificate path is empty")
	}

	info, err := CheckCertificateExpiration(certPath, zap.NewNop())
	if err != nil {
		return -1, err
	}

	return info.DaysUntil, nil
}
