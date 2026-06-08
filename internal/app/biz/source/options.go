package source

type sourceOption struct {
	populateContentRef bool
	forDownload        bool
}

func newSourceOption(opts ...SourceOption) *sourceOption {
	var opt sourceOption
	for _, option := range opts {
		option(&opt)
	}

	return &opt
}

type SourceOption func(o *sourceOption)

func WithContentRefUrl(b bool) SourceOption {
	return func(o *sourceOption) {
		o.populateContentRef = b
	}
}

func WithForDownload(b bool) SourceOption {
	return func(o *sourceOption) {
		o.forDownload = b
	}
}
