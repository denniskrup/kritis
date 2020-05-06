/*
Copyright 2020 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cryptolib

import (
	"errors"
	"fmt"

	"github.com/golang/glog"
)

// Verifier contains methods to validate an Attestation.
type Verifier interface {
	// VerifyAttestation verifies whether an Attestation satisfies at least one
	// of the public keys under an image. This function finds the public key
	// whose ID matches the attestation's PublicKeyID, and uses this key to
	// verify the signature.
	VerifyAttestation(att *Attestation) error
}

// PublicKey stores public key material for all key types.
type PublicKey struct {
	// KeyType stores the type of the public key, one of Pgp, Pkix, or Jwt.
	KeyType KeyType
	// KeyData holds the raw key material which can verify a signature.
	KeyData []byte
	// ID uniquely identifies this public key. For PGP, this should be the
	// OpenPGP RFC4880 V4 fingerprint of the key.
	ID string
}

// NewPublicKey creates a new PublicKey. `keyType` contains the type of the
// public key, one of Pgp, Pkix or Jwt. `keyData` contains the raw key
// material. `keyID` contains a unique identifier for the public key. For PGP,
// this should be the OpenPGP RFC4880 V4 fingerprint of the key.
func NewPublicKey(keyType KeyType, keyData []byte, keyID string) PublicKey {
	return PublicKey{
		KeyType: keyType,
		KeyData: keyData,
		ID:      keyID,
	}
}

type pkixVerifier interface {
	verifyPkix(signature []byte, payload []byte, publicKey []byte) error
}

type pgpVerifier interface {
	verifyPgp(signature, publicKey []byte) ([]byte, error)
}

type jwtVerifier interface {
	verifyJwt(signature []byte, publicKey []byte) ([]byte, error)
}

type authenticatedAuthChecker interface {
	checkAuthenticatedAttestation(actual authenticatedAttestation, imageDigest string) error
}

type verifier struct {
	ImageDigest string
	// PublicKeys is an index of public keys by their ID.
	PublicKeys map[string]PublicKey

	// Interfaces for testing
	pkixVerifier
	pgpVerifier
	jwtVerifier
	authenticatedAuthChecker
}

// NewVerifier creates a Verifier interface for verifying Attestations.
// `imageDigest` contains the digest of the image that was signed over. This
// should be provided directly by the policy evaluator, NOT by the Attestation.
// `publicKeySet` contains a list of PublicKeys that the Verifier will use to
// try to verify an Attestation.
func NewVerifier(imageDigest string, publicKeySet []PublicKey) (Verifier, error) {
	keyMap := indexPublicKeysByID(publicKeySet)
	return &verifier{
		ImageDigest:              imageDigest,
		PublicKeys:               keyMap,
		pkixVerifier:             pkixVerifierImpl{},
		pgpVerifier:              pgpVerifierImpl{},
		jwtVerifier:              jwtVerifierImpl{},
		authenticatedAuthChecker: attAuthChecker{},
	}, nil
}

func indexPublicKeysByID(publicKeyset []PublicKey) map[string]PublicKey {
	keyMap := map[string]PublicKey{}
	for _, publicKey := range publicKeyset {
		if _, ok := keyMap[publicKey.ID]; ok {
			glog.Warningf("Key with ID %q already exists in publicKeySet. Overwriting previous key.", publicKey.ID)
		}
		keyMap[publicKey.ID] = publicKey
	}
	return keyMap
}

// VerifyAttestation verifies an Attestation. See Verifier for more details.
func (v *verifier) VerifyAttestation(att *Attestation) error {
	// Extract the public key from `publicKeySet` whose ID matches the one in
	// `att`.
	publicKey, ok := v.PublicKeys[att.PublicKeyID]
	if !ok {
		return fmt.Errorf("no public key with ID %q found", att.PublicKeyID)
	}

	var err error
	payload := []byte{}
	switch publicKey.KeyType {
	case Pkix:
		err = v.verifyPkix(att.Signature, att.SerializedPayload, publicKey.KeyData)
		payload = att.SerializedPayload
	case Pgp:
		payload, err = v.verifyPgp(att.Signature, publicKey.KeyData)
	case Jwt:
		payload, err = v.verifyJwt(att.Signature, publicKey.KeyData)
	default:
		return errors.New("signature uses an unsupported key mode")
	}
	if err != nil {
		return err
	}

	// Extract the payload into an AuthenticatedAttestation, whose contents we
	// can trust.
	actual := formAuthenticatedAttestation(payload)
	return v.checkAuthenticatedAttestation(actual, v.ImageDigest)
}

type pkixVerifierImpl struct{}

func (v pkixVerifierImpl) verifyPkix(signature []byte, payload []byte, publicKey []byte) error {
	return errors.New("verify pkix not implemented")
}

type jwtVerifierImpl struct{}

func (v jwtVerifierImpl) verifyJwt(signature []byte, publicKey []byte) ([]byte, error) {
	return []byte{}, errors.New("verify jwt not implemented")
}

// authenticatedAttestation contains data that is extracted from an Attestation
// only after its signature has been verified. The contents of an Attestation
// payload should never be analyzed directly, as it may or may not be verified.
// Instead, these should be extracted into an AuthenticatedAttestation and
// analyzed from there.
// NOTE: The concept and usefulness of an AuthenticatedAttestation are still
// under discussion and is subject to change.
type authenticatedAttestation struct {
	ImageDigest string
}

func formAuthenticatedAttestation(payload []byte) authenticatedAttestation {
	return authenticatedAttestation{}
}

type attAuthChecker struct{}

// Check that the data within the Attestation payload matches what we expect.
// NOTE: This is a simple comparison for plain attestations, but it would be
// more complex for rich attestations.
func (c attAuthChecker) checkAuthenticatedAttestation(actual authenticatedAttestation, imageDigest string) error {
	if actual.ImageDigest != imageDigest {
		return errors.New("invalid payload for authenticated attestation")
	}
	return nil
}
