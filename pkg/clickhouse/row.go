package clickhouse

type pesudoErrRow struct {
	err error
}

func (p pesudoErrRow) Err() error {
	return p.err
}

func (p pesudoErrRow) Scan(dest ...any) error {
	return p.err
}

func (p pesudoErrRow) ScanStruct(dest any) error {
	return p.err
}
