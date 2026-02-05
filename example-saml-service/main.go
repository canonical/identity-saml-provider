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

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
)

const (
	serviceURL               = "http://localhost:8083"
	defaultIDPMetadataURLStr = "http://localhost:8082/saml/metadata"
	listenPort               = ":8083"
)

func hello(w http.ResponseWriter, r *http.Request) {
	s := samlsp.SessionFromContext(r.Context())
	if s == nil {
		return
	}
	sa, ok := s.(samlsp.SessionWithAttributes)
	if !ok {
		return
	}
	attrs := sa.GetAttributes()
	displayName := attrs.Get("cn")
	fmt.Fprintf(w, "Hello, %s!\n", displayName)
	fmt.Fprintf(w, "\nAll attributes:\n")
	for name, values := range attrs {
		fmt.Fprintf(w, "%s: %v\n", name, values)
	}
}

func fetchIDPMetadataWithRetry(idpURL *url.URL) *saml.EntityDescriptor {
	maxWait := 30 * time.Second
	initialDelay := time.Second
	delay := initialDelay
	elapsed := time.Duration(0)

	for {
		idpMetadata, err := samlsp.FetchMetadata(context.Background(), http.DefaultClient, *idpURL)
		if err == nil {
			return idpMetadata
		}

		if elapsed+delay > maxWait {
			panic(fmt.Sprintf("Failed to fetch IdP metadata after %v: %v", maxWait, err))
		}

		log.Printf("Failed to fetch IdP metadata: %v. Retrying in %v...", err, delay)
		time.Sleep(delay)
		elapsed += delay
		delay = delay * 2
		if delay > maxWait-elapsed {
			delay = maxWait - elapsed
		}
	}
}

func main() {
	certPath := flag.String("cert", "etc/certs/myservice.crt", "Path to the SAML service certificate")
	keyPath := flag.String("key", "etc/certs/myservice.key", "Path to the SAML service private key")
	idpMetadataURLStr := flag.String("idp-metadata-url", defaultIDPMetadataURLStr, "URL to the IdP metadata")
	flag.Parse()

	keyPair, err := tls.LoadX509KeyPair(*certPath, *keyPath)
	if err != nil {
		panic(err) // TODO handle error
	}
	keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		panic(err) // TODO handle error
	}

	idpMetadataURL, err := url.Parse(*idpMetadataURLStr)
	if err != nil {
		panic(err) // TODO handle error
	}
	idpMetadata := fetchIDPMetadataWithRetry(idpMetadataURL)
	log.Printf("Fetched IdP metadata from %s\n", *idpMetadataURLStr)

	rootURL, err := url.Parse(serviceURL)
	if err != nil {
		panic(err) // TODO handle error
	}

	samlSP, _ := samlsp.New(samlsp.Options{
		URL:         *rootURL,
		Key:         keyPair.PrivateKey.(*rsa.PrivateKey),
		Certificate: keyPair.Leaf,
		IDPMetadata: idpMetadata,
	})
	app := http.HandlerFunc(hello)
	http.Handle("/hello", samlSP.RequireAccount(app))
	http.Handle("/saml/", samlSP)
	http.ListenAndServe(listenPort, nil)
}
