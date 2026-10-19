package vo

// Visibility 控制除 owner 和显式成员外的访问范围。
type Visibility string

const (
	VisibilityRestricted Visibility = "restricted"
	VisibilityPublic     Visibility = "public"
)

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
	return v
}
