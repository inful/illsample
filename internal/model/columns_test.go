package model

import (
	"reflect"
	"testing"
)

func TestColumnByName(t *testing.T) {
	tests := []struct {
		in   string
		want string // canonical name, or "" if no match
	}{
		// Canonical names match themselves.
		{"Sample_ID", "Sample_ID"},
		{"Sample_Name", "Sample_Name"},
		{"Sample_Plate", "Sample_Plate"},
		{"Sample_Well", "Sample_Well"},
		{"I7_Index_ID", "I7_Index_ID"},
		{"index", "index"},
		{"I5_Index_ID", "I5_Index_ID"},
		{"index2", "index2"},
		{"Sample_Project", "Sample_Project"},
		{"Description", "Description"},
		// Aliases resolve to the canonical name.
		{"SampleID", "Sample_ID"},
		{"SampleName", "Sample_Name"},
		{"SamplePlate", "Sample_Plate"},
		{"SampleWell", "Sample_Well"},
		{"I7IndexID", "I7_Index_ID"},
		{"I5IndexID", "I5_Index_ID"},
		{"SampleProject", "Sample_Project"},
		// Case-insensitive.
		{"sample_id", "Sample_ID"},
		{"SAMPLE_ID", "Sample_ID"},
		{"i7indexid", "I7_Index_ID"},
		// Unknown column.
		{"CustomColumn", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			c := ColumnByName(tt.in)
			got := ""
			if c != nil {
				got = c.CanonicalName()
			}
			if got != tt.want {
				t.Errorf("ColumnByName(%q).CanonicalName() = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestColumnSetGet(t *testing.T) {
	// Set and Get must be inverses for every typed field.
	cases := []struct {
		col  string
		set  func(*Sample, string)
		get  func(*Sample) string
		want string
	}{
		{"Sample_ID", func(s *Sample, v string) { s.SampleID = v }, func(s *Sample) string { return s.SampleID }, "id1"},
		{"Sample_Name", func(s *Sample, v string) { s.SampleName = v }, func(s *Sample) string { return s.SampleName }, "name1"},
		{"Sample_Plate", func(s *Sample, v string) { s.SamplePlate = v }, func(s *Sample) string { return s.SamplePlate }, "p1"},
		{"Sample_Well", func(s *Sample, v string) { s.SampleWell = v }, func(s *Sample) string { return s.SampleWell }, "A1"},
		{"I7_Index_ID", func(s *Sample, v string) { s.I7IndexID = v }, func(s *Sample) string { return s.I7IndexID }, "UDI1"},
		{"index", func(s *Sample, v string) { s.Index = v }, func(s *Sample) string { return s.Index }, "ATCG"},
		{"I5_Index_ID", func(s *Sample, v string) { s.I5IndexID = v }, func(s *Sample) string { return s.I5IndexID }, "UDI2"},
		{"index2", func(s *Sample, v string) { s.Index2 = v }, func(s *Sample) string { return s.Index2 }, "GCTA"},
		{"Sample_Project", func(s *Sample, v string) { s.SampleProject = v }, func(s *Sample) string { return s.SampleProject }, "P1"},
		{"Description", func(s *Sample, v string) { s.Description = v }, func(s *Sample) string { return s.Description }, "note"},
	}
	for _, tt := range cases {
		t.Run(tt.col, func(t *testing.T) {
			s := &Sample{}
			tt.set(s, tt.want)
			if got := tt.get(s); got != tt.want {
				t.Errorf("get after set = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestColumnByName_SetAndGet(t *testing.T) {
	// Looking up a column by alias and then using its Set / Get must
	// hit the same field as the canonical lookup.
	s := &Sample{}
	c := ColumnByName("SampleID")
	if c == nil {
		t.Fatal("ColumnByName(SampleID) = nil")
	}
	c.Set(s, "id-via-alias")
	if s.SampleID != "id-via-alias" {
		t.Errorf("SampleID = %q, want id-via-alias", s.SampleID)
	}
	if got := c.Get(s); got != "id-via-alias" {
		t.Errorf("Get = %q, want id-via-alias", got)
	}
}

func TestCanonicalColumnNames(t *testing.T) {
	got := CanonicalColumnNames()
	want := []string{
		"Sample_ID", "Sample_Name", "Sample_Plate", "Sample_Well",
		"I7_Index_ID", "index", "I5_Index_ID", "index2",
		"Sample_Project", "Description",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CanonicalColumnNames() = %v, want %v", got, want)
	}
}

func TestColumnCanonicalName_NilAndEmpty(t *testing.T) {
	// Defensive: CanonicalName must not panic on a nil or zero-value
	// column. The writer's canonicalAlias path can call this when a
	// caller passes an empty name.
	var c *Column
	if got := c.CanonicalName(); got != "" {
		t.Errorf("(*Column)(nil).CanonicalName() = %q, want \"\"", got)
	}
	zero := &Column{}
	if got := zero.CanonicalName(); got != "" {
		t.Errorf("(&Column{}).CanonicalName() = %q, want \"\"", got)
	}
}
