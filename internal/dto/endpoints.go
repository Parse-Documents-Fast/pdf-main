package dto

// Downstream endpoint paths.
//
// pdf-main is the only service that talks to the other services over HTTP, so
// it defines these routes as the provisional inter-service contract. They must
// be reconciled with each downstream repo when it defines its real API
// (coordination point, see docs/spec.md §Coordinación).
const (
	// PathValidatorValidate is pdf-validator's classification endpoint
	// (POST, body ValidateRequest).
	PathValidatorValidate = "/validate"

	// PathConverterConvert is pdf-converter's Markdown → PDF endpoint
	// (POST, body ConvertRequest).
	PathConverterConvert = "/convert"

	// PathPersistDocuments is pdf-persistance's collection endpoint
	// (POST create, GET list).
	PathPersistDocuments = "/documents"

	// PathPersistFindByChecksum is pdf-persistance's duplicate lookup
	// endpoint (GET ?checksum=...).
	PathPersistFindByChecksum = "/documents/by-checksum"
)
