package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

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

func main() {
	keyPair, err := tls.LoadX509KeyPair("myservice.crt", "myservice.key")
	if err != nil {
		panic(err) // TODO handle error
	}
	keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		panic(err) // TODO handle error
	}

	idpMetadataURLStr := os.Getenv("IDP_METADATA_URL")
	if idpMetadataURLStr == "" {
		idpMetadataURLStr = defaultIDPMetadataURLStr
	}

	idpMetadataURL, err := url.Parse(idpMetadataURLStr)
	if err != nil {
		panic(err) // TODO handle error
	}
	idpMetadata, err := samlsp.FetchMetadata(context.Background(), http.DefaultClient,
		*idpMetadataURL)
	if err != nil {
		panic(err) // TODO handle error
	}
	log.Printf("Fetched IdP metadata from %s\n", idpMetadataURLStr)

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
