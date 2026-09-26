package queue

// Redis Stream names (ADR-0004 + ADR-0005). Only the extraction pipeline
// exists; queue:conversion* were removed by ADR-0005.
const (
	streamExtraction        = "queue:extraction"
	streamExtractionResults = "queue:extraction-results"
)
