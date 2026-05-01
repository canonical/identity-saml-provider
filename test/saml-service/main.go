package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
)

const (
	serviceURL               = "http://localhost:8083"
	defaultIDPMetadataURLStr = "http://localhost:8082/saml/metadata"
	listenPort               = ":8083"
)

func hello(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	s := samlsp.SessionFromContext(r.Context())
	if s == nil {
		return
	}
	sa, ok := s.(samlsp.SessionWithAttributes)
	if !ok {
		return
	}
	attrs := sa.GetAttributes()

	// Try mapped attribute names first, then fall back to defaults
	displayName := attrs.Get("displayName")
	if displayName == "" {
		displayName = attrs.Get("cn")
	}
	if displayName == "" {
		displayName = attrs.Get("name")
	}
	if displayName == "" {
		displayName = attrs.Get("mail")
	}
	if displayName == "" {
		displayName = attrs.Get("email")
	}
	if displayName == "" {
		displayName = "unknown"
	}

	fmt.Fprintf(w, "Hello, %s!\n", displayName)

	// Display NameID from the JWT session Subject field
	if jwtSession, ok := s.(samlsp.JWTSessionClaims); ok && jwtSession.Subject != "" {
		fmt.Fprintf(w, "\nNameID: %s\n", jwtSession.Subject)
	}

	fmt.Fprintf(w, "\nAll attributes:\n")
	for name, values := range attrs {
		fmt.Fprintf(w, "%s: %v\n", name, values)
	}
}

func fetchIDPMetadataWithRetry(idpURL *url.URL) *saml.EntityDescriptor {
	totalCtx, totalCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer totalCancel()

	var idpMetadata *saml.EntityDescriptor

	err := retry.Do(
		func() error {
			// Apply the 5-second timeout per attempt as recommended
			reqCtx, reqCancel := context.WithTimeout(totalCtx, 5*time.Second)
			defer reqCancel()

			var fetchErr error
			idpMetadata, fetchErr = samlsp.FetchMetadata(reqCtx, http.DefaultClient, *idpURL)
			return fetchErr
		},
		retry.Context(totalCtx),
		retry.Attempts(0),
		retry.Delay(time.Second),            // Initial delay
		retry.DelayType(retry.BackOffDelay), // Exponential backoff
		retry.OnRetry(func(n uint, err error) {
			log.Printf("Attempt %d: Failed to fetch IdP metadata: %v. Retrying...", n+1, err)
		}),
	)

	if err != nil {
		log.Fatalf("Failed to fetch IdP metadata after retries: %v", err)
	}

	return idpMetadata
}

func main() {
	certPath := flag.String("cert", "etc/certs/myservice.crt", "Path to the SAML service certificate")
	keyPath := flag.String("key", "etc/certs/myservice.key", "Path to the SAML service private key")
	idpMetadataURLStr := flag.String("idp-metadata-url", defaultIDPMetadataURLStr, "URL to the IdP metadata")
	flag.Parse()

	keyPair, err := tls.LoadX509KeyPair(*certPath, *keyPath)
	if err != nil {
		panic(fmt.Sprintf("failed loading key pair: %v", err))
	}
	keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		panic(fmt.Sprintf("failed parsing certificate: %v", err))
	}

	idpMetadataURL, err := url.Parse(*idpMetadataURLStr)
	if err != nil {
		log.Fatalf("failed parsing IdP metadata URL: %v", err)
	}
	idpMetadata := fetchIDPMetadataWithRetry(idpMetadataURL)
	log.Printf("Fetched IdP metadata from %s\n", *idpMetadataURLStr)

	rootURL, err := url.Parse(serviceURL)
	if err != nil {
		log.Fatalf("failed parsing service URL: %v", err)
	}

	samlSP, err := samlsp.New(samlsp.Options{
		URL:         *rootURL,
		Key:         keyPair.PrivateKey.(*rsa.PrivateKey),
		Certificate: keyPair.Leaf,
		IDPMetadata: idpMetadata,
	})
	if err != nil {
		log.Fatalf("failed to initialize SAML SP: %v", err)
	}

	app := http.HandlerFunc(hello)
	http.Handle("/hello", samlSP.RequireAccount(app))
	http.Handle("/saml/", samlSP)

	log.Printf("Starting Example SAML Service at %s/hello\n", serviceURL)

	log.Fatal(http.ListenAndServe(listenPort, nil))
}
