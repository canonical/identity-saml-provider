-- Hydra
CREATE USER hydra WITH PASSWORD 'hydra';

CREATE DATABASE hydra;
GRANT ALL PRIVILEGES ON DATABASE hydra TO hydra;
ALTER DATABASE hydra OWNER TO hydra;

-- Kratos
CREATE USER kratos WITH PASSWORD 'kratos';

CREATE DATABASE kratos;
GRANT ALL PRIVILEGES ON DATABASE kratos TO kratos;
ALTER DATABASE kratos OWNER TO kratos;

-- SAML Provider
CREATE USER saml_provider WITH PASSWORD 'saml_provider';

CREATE DATABASE saml_provider;
GRANT ALL PRIVILEGES ON DATABASE saml_provider TO saml_provider;
ALTER DATABASE saml_provider OWNER TO saml_provider;

-- SAML Provider Tests
CREATE DATABASE saml_provider_tests;
GRANT ALL PRIVILEGES ON DATABASE saml_provider_tests TO saml_provider;
ALTER DATABASE saml_provider_tests OWNER TO saml_provider;
