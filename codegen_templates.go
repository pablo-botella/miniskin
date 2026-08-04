package miniskin

import _ "embed"

//go:embed templates/bucket_default.go.tmpl
var defaultBucketTmpl string

//go:embed templates/bucket_mux.go.tmpl
var muxBucketTmpl string

//go:embed templates/bucket_muxblob.go.tmpl
var muxblobBucketTmpl string

//go:embed templates/embed_default.go.tmpl
var defaultEmbedTmpl string

// namedBucketTemplates maps "miniskin::name" to built-in bucket templates.
var namedBucketTemplates = map[string]string{
	"miniskin::default": defaultBucketTmpl,
	"miniskin::mux":     muxBucketTmpl,
	"miniskin::muxblob": muxblobBucketTmpl,
}

// namedEmbedTemplates maps "miniskin::name" to built-in embed templates.
var namedEmbedTemplates = map[string]string{
	"miniskin::default": defaultEmbedTmpl,
}
