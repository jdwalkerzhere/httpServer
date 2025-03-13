	package auth

	import (
		"math"
		"testing"
		"time"

		"github.com/golang-jwt/jwt/v5"
		"github.com/google/uuid"
	)

	func TestHashPassword(t *testing.T) {
		t.Run("successfully hashes password", func(t *testing.T) {
			password := "securePassword123"
			
			hash, err := HashPassword(password)
			
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			if len(hash) == 0 {
				t.Errorf("Expected non-empty hash, got empty string")
			}
			if password == hash {
				t.Errorf("Expected hash to be different from password, got: %s", hash)
			}
		})
		
		t.Run("generates different hashes for same password", func(t *testing.T) {
			password := "securePassword123"
			
			hash1, err := HashPassword(password)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			
			hash2, err := HashPassword(password)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			
			if hash1 == hash2 {
				t.Errorf("Expected different hashes due to salting, got the same hash: %s", hash1)
			}
		})
	}

	func TestCheckPasswordHash(t *testing.T) {
		t.Run("successful password verification", func(t *testing.T) {
			password := "securePassword123"
			
			hash, err := HashPassword(password)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			
			err = CheckPasswordHash(password, hash)
			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
		
		t.Run("fails with incorrect password", func(t *testing.T) {
			password := "securePassword123"
			wrongPassword := "wrongPassword456"
			
			hash, err := HashPassword(password)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			
			err = CheckPasswordHash(wrongPassword, hash)
			if err == nil {
				t.Errorf("Expected error with wrong password, got nil")
			}
		})
	}

	func TestMakeJWT(t *testing.T) {
		t.Run("successfully generates JWT token", func(t *testing.T) {
			userID := uuid.New()
			tokenSecret := "test-secret-key"
			expiresIn := 24 * time.Hour
			
			token, err := MakeJWT(userID, tokenSecret, expiresIn)
			
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			if len(token) == 0 {
				t.Errorf("Expected non-empty token, got empty string")
			}
		})
		
		t.Run("token contains expected claims", func(t *testing.T) {
			userID := uuid.New()
			tokenSecret := "test-secret-key"
			expiresIn := 24 * time.Hour
			
			token, err := MakeJWT(userID, tokenSecret, expiresIn)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			
			// Parse token to verify claims
			parsedToken, err := jwt.ParseWithClaims(token, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
				return []byte(tokenSecret), nil
			})
			
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			claims, ok := parsedToken.Claims.(*jwt.RegisteredClaims)
			if !ok {
				t.Fatalf("Expected claims to be of type *jwt.RegisteredClaims")
			}
			
			if claims.Issuer != "chirpy" {
				t.Errorf("Expected issuer to be 'chirpy', got: %s", claims.Issuer)
			}
			if claims.Subject != userID.String() {
				t.Errorf("Expected subject to be '%s', got: %s", userID.String(), claims.Subject)
			}
			if claims.IssuedAt == nil {
				t.Errorf("Expected IssuedAt to be non-nil")
			}
			if claims.ExpiresAt == nil {
				t.Errorf("Expected ExpiresAt to be non-nil")
			}
			
			// Check expiration is roughly as expected (allowing 1 second tolerance)
			expectedExpiry := time.Now().Add(expiresIn).Unix()
			actualExpiry := claims.ExpiresAt.Unix()
			tolerance := float64(1)
			if math.Abs(float64(expectedExpiry-actualExpiry)) > tolerance {
				t.Errorf("Expected expiry to be around %d, got: %d (difference: %f)", 
					expectedExpiry, actualExpiry, math.Abs(float64(expectedExpiry-actualExpiry)))
			}
		})
	}

	func TestValidateJWT(t *testing.T) {
		t.Run("successfully validates token", func(t *testing.T) {
			userID := uuid.New()
			tokenSecret := "test-secret-key"
			expiresIn := 24 * time.Hour
			
			token, err := MakeJWT(userID, tokenSecret, expiresIn)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			
			extractedUserID, err := ValidateJWT(token, tokenSecret)
			
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			if extractedUserID != userID {
				t.Errorf("Expected user ID to be %v, got: %v", userID, extractedUserID)
			}
		})
		
		t.Run("fails with invalid signature", func(t *testing.T) {
			userID := uuid.New()
			tokenSecret := "test-secret-key"
			wrongSecret := "wrong-secret-key"
			expiresIn := 24 * time.Hour
			
			token, err := MakeJWT(userID, tokenSecret, expiresIn)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			
			extractedUserID, err := ValidateJWT(token, wrongSecret)
			
			if err == nil {
				t.Errorf("Expected error, got nil")
			}
			if extractedUserID != uuid.Nil {
				t.Errorf("Expected user ID to be Nil, got: %v", extractedUserID)
			}
		})

						t.Run("fails with expired token", func(t *testing.T) {
							userID := uuid.New()
							tokenSecret := "test-secret-key"
							expiresIn := -1 * time.Hour // expired 1 hour ago
							
							token, err := MakeJWT(userID, tokenSecret, expiresIn)
							if err != nil {
								t.Fatalf("Expected no error, got: %v", err)
							}
							
							extractedUserID, err := ValidateJWT(token, tokenSecret)
							
							if err == nil {
								t.Errorf("Expected error, got nil")
							}
							if extractedUserID != uuid.Nil {
								t.Errorf("Expected user ID to be Nil, got: %v", extractedUserID)
							}
		})
		
		t.Run("fails with malformed token", func(t *testing.T) {
			malformedToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.malformed-token"
			tokenSecret := "test-secret-key"
			
			extractedUserID, err := ValidateJWT(malformedToken, tokenSecret)
			
			if err == nil {
				t.Errorf("Expected error, got nil")
			}
			if extractedUserID != uuid.Nil {
				t.Errorf("Expected user ID to be Nil, got: %v", extractedUserID)
			}
			if extractedUserID != uuid.Nil {
				t.Errorf("Expected user ID to be Nil, got: %v", extractedUserID)
			}
		})
	}
