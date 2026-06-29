package markdown

const (
	MetaHeadingH1Key = "md_h1"
	MetaHeadingH2Key = "md_h2"
	MetaHeadingH3Key = "md_h3"
	MetaHeadingH4Key = "md_h4"
	MetaHeadingH5Key = "md_h5"
	MetaHeadingH6Key = "md_h6"

	MetaChunkByteStartKey = "md_chunk_byte_start"
	MetaChunkByteEndKey   = "md_chunk_byte_end"
	MetaChunkRuneStartKey = "md_chunk_rune_start"
	MetaChunkRuneEndKey   = "md_chunk_rune_end"
)

var headingMetaKeys = [6]string{
	MetaHeadingH1Key, MetaHeadingH2Key, MetaHeadingH3Key,
	MetaHeadingH4Key, MetaHeadingH5Key, MetaHeadingH6Key,
}

type headingEntry struct {
	level int
	title string
	br    byteRange
}

type headingStack struct {
	entries []headingEntry
}

func newHeadingStack() *headingStack {
	return &headingStack{}
}

func (s *headingStack) push(e headingEntry) {
	for len(s.entries) > 0 && s.entries[len(s.entries)-1].level >= e.level {
		s.entries = s.entries[:len(s.entries)-1]
	}
	s.entries = append(s.entries, e)
}

func (s *headingStack) toMetaData() map[string]any {
	meta := make(map[string]any)
	for _, e := range s.entries {
		if e.level < 1 || e.level > 6 {
			continue
		}
		meta[headingMetaKeys[e.level-1]] = e.title
	}
	return meta
}

func setPositionMeta(meta map[string]any, byteStart, byteEnd, runeStart, runeEnd int) {
	if meta == nil {
		return
	}
	meta[MetaChunkByteStartKey] = byteStart
	meta[MetaChunkByteEndKey] = byteEnd
	meta[MetaChunkRuneStartKey] = runeStart
	meta[MetaChunkRuneEndKey] = runeEnd
}
