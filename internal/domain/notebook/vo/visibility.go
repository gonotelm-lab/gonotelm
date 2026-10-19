package vo

// Visibility 控制除 owner 和显式成员外的访问范围。
type Visibility string

const (
	VisibilityRestricted Visibility = "restricted"
	VisibilityPublic     Visibility = "public"
)

func (v Visibility) String() string {
	return string(v)
}

func (v Visibility) IsValid() bool {
	return v.IsRestricted() || v.IsPublic()
}

func (v Visibility) IsRestricted() bool {
	return v == VisibilityRestricted
}

func (v Visibility) IsPublic() bool {
	return v == VisibilityPublic
}

func NormalizeVisibility(v Visibility) Visibility {
	if v == "" {
		return VisibilityRestricted
	}
	if !v.IsValid() {
		return VisibilityRestricted
	}

	return v
}

func NewVisibility(v string) Visibility {
	vv := Visibility(v)
	return NormalizeVisibility(vv)
}
