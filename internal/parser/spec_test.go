package parser

import (
	"strings"
	"testing"

	"github.com/inful/illsample/internal/model"
)

// The tests in this file check the layouts that Illumina documents:
// the "Sequencing Sample Sheet Format Specifications" (Pub. No.
// 970-2017-004-A) and the "MiSeq Sample Sheet Quick Reference Guide"
// (Part # 15028392 Rev. J).

func TestParseV2_SpecReadsCycleCounts(t *testing.T) {
	// The specification holds one cycle count per line: read 1, then
	// read 2. The index reads are not part of this section.
	input := "[Header]\n" +
		"Date,2007-01-26\n" +
		"[Reads]\n" +
		"151\n" +
		"151\n" +
		"[Data]\n" +
		"Sample_ID,index\n" +
		"id1,ATCG\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Reads == nil {
		t.Fatal("Reads is nil")
	}
	if sheet.Reads.Read1Cycles != 151 {
		t.Errorf("Read1Cycles = %d, want 151", sheet.Reads.Read1Cycles)
	}
	if sheet.Reads.Read2Cycles != 151 {
		t.Errorf("Read2Cycles = %d, want 151", sheet.Reads.Read2Cycles)
	}
}

func TestParseV2_SpecReadsSingleEnd(t *testing.T) {
	input := "[Reads]\n151\n[Data]\nSample_ID,index\nid1,ATCG\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Reads.Read1Cycles != 151 {
		t.Errorf("Read1Cycles = %d, want 151", sheet.Reads.Read1Cycles)
	}
	if sheet.Reads.Read2Cycles != 0 {
		t.Errorf("Read2Cycles = %d, want 0", sheet.Reads.Read2Cycles)
	}
}

func TestParseV2_SpecReadsRejectsThirdCount(t *testing.T) {
	// The specification allows one count per read; index reads do not
	// belong in this section.
	input := "[Reads]\n151\n151\n8\n[Data]\nSample_ID,index\nid1,ATCG\n"
	if _, err := Parse(strings.NewReader(input)); err == nil {
		t.Fatal("Parse: expected an error for a third cycle count")
	}
}

func TestParseV2_SpecSettingsPairs(t *testing.T) {
	// Both specifications write the settings as "Key,Value" pairs, one
	// per line.
	input := "[Settings]\n" +
		"Adapter,AGATCGGAAGAGCACACGTCTGAACTCCAGTCA\n" +
		"AdapterRead2,AGATCGGAAGAGCGTCGTGTAGGGAAAGAGT\n" +
		"[Data]\n" +
		"Sample_ID,index\n" +
		"id1,ATCG\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Settings == nil {
		t.Fatal("Settings is nil")
	}
	if got, want := sheet.Settings.Adapter, "AGATCGGAAGAGCACACGTCTGAACTCCAGTCA"; got != want {
		t.Errorf("Adapter = %q, want %q", got, want)
	}
	if got, want := sheet.Settings.AdapterRead2, "AGATCGGAAGAGCGTCGTGTAGGGAAAGAGT"; got != want {
		t.Errorf("AdapterRead2 = %q, want %q", got, want)
	}
}

func TestParseV2_SpecManifestsPairs(t *testing.T) {
	// The specification writes the manifests as "Key,Filename" pairs;
	// the key is what the Data section's Manifest column refers to.
	input := "[Manifests]\n" +
		"A,TruSeqAmpliconManifest-1.txt\n" +
		"B,TruSeqAmpliconManifest-2.txt\n" +
		"[Data]\n" +
		"Sample_ID,index,Manifest\n" +
		"id1,ATCG,A\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(sheet.Manifests) != 2 {
		t.Fatalf("len(Manifests) = %d, want 2", len(sheet.Manifests))
	}
	if got, want := sheet.Manifests[0].Key, "A"; got != want {
		t.Errorf("Manifests[0].Key = %q, want %q", got, want)
	}
	if got, want := sheet.Manifests[0].Filename, "TruSeqAmpliconManifest-1.txt"; got != want {
		t.Errorf("Manifests[0].Filename = %q, want %q", got, want)
	}
	if got, want := sheet.Manifests[1].Key, "B"; got != want {
		t.Errorf("Manifests[1].Key = %q, want %q", got, want)
	}
	if got, want := sheet.Manifests[1].Filename, "TruSeqAmpliconManifest-2.txt"; got != want {
		t.Errorf("Manifests[1].Filename = %q, want %q", got, want)
	}
}

func TestParseV2_SectionLabelWithPaddedCommas(t *testing.T) {
	// A section label may be padded with commas, and so may the
	// records below it.
	input := "[Header],,,,,,\n" +
		"Date,2007-01-26,,,,,\n" +
		"[Data]\n" +
		"Sample_ID,index\n" +
		"id1,ATCG\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Format != model.FormatV2 {
		t.Errorf("Format = %v, want v2", sheet.Format)
	}
	if len(sheet.Data) != 1 {
		t.Errorf("len(Data) = %d, want 1", len(sheet.Data))
	}
	if got, want := sheet.Header["Date"], "2007-01-26"; got != want {
		t.Errorf("Header[Date] = %q, want %q (got %v)", got, want, sheet.Header)
	}
}

func TestParseV2_BlankLinesIgnored(t *testing.T) {
	// A line that holds nothing but commas and whitespace is valid but
	// ignored; it must not become a sample row.
	input := "[Data]\n" +
		"Sample_ID,index\n" +
		"id1,ATCG\n" +
		",,\n" +
		"   \n" +
		"id2,GCTA\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(sheet.Data) != 2 {
		t.Errorf("len(Data) = %d, want 2", len(sheet.Data))
	}
}

func TestParseV1_BlankLinesIgnored(t *testing.T) {
	input := "Sample_ID,index\n" +
		"id1,ATCG\n" +
		",,,\n" +
		"id2,GCTA\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(sheet.Data) != 2 {
		t.Errorf("len(Data) = %d, want 2", len(sheet.Data))
	}
}

func TestParseV2_MiSeqColumnSpellings(t *testing.T) {
	// The MiSeq guide writes the identifier column "SampleID" and the
	// index columns "Index" and "Index2".
	input := "[Header]\n" +
		"Date,2007-01-26\n" +
		"[Reads]\n" +
		"151\n" +
		"151\n" +
		"[Settings]\n" +
		"Adapter,AGATCGGAAGAGCACACGTCTGAACTCCAGTCA\n" +
		"[Data]\n" +
		"SampleID,Sample_Name,Index,Index2\n" +
		"A10001,Sample_A,ATTACTCG,TATAGCCT\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(sheet.Data) != 1 {
		t.Fatalf("len(Data) = %d, want 1", len(sheet.Data))
	}
	got := sheet.Data[0]
	if got.SampleID != "A10001" {
		t.Errorf("SampleID = %q, want A10001", got.SampleID)
	}
	if got.SampleName != "Sample_A" {
		t.Errorf("SampleName = %q, want Sample_A", got.SampleName)
	}
	if got.Index != "ATTACTCG" {
		t.Errorf("Index = %q, want ATTACTCG", got.Index)
	}
	if got.Index2 != "TATAGCCT" {
		t.Errorf("Index2 = %q, want TATAGCCT", got.Index2)
	}
}

func TestParseV2_SpecFullSheet(t *testing.T) {
	// The TruSeq Amplicon example of the specification, end to end.
	input := "[Header]\n" +
		"Date,2017-04-05\n" +
		"Workflow,Custom Amplicon\n" +
		"Application,TruSeq Amplicon\n" +
		"Assay,TruSeq Amplicon\n" +
		"Description,\n" +
		"Chemistry,Amplicon\n" +
		"[Manifests]\n" +
		"A,TruSeqAmpliconManifest-1.txt\n" +
		"B,TruSeqAmpliconManifest-2.txt\n" +
		"[Reads]\n" +
		"151\n" +
		"151\n" +
		"[Settings]\n" +
		"VariantFilterQualityCutoff,30\n" +
		"outputgenomevcf,FALSE\n" +
		"[Data]\n" +
		"Sample_ID,Sample_Name,I7_Index_ID,index,I5_Index_ID,index2,Manifest,GenomeFolder\n" +
		"A10001,Sample_A,A701,ATCACGAC,A501,TGAACCTT,A,Homo_sapiens\\UCSC\\hg19\n" +
		"A10002,Sample_B,A702,ACAGTGGT,A501,TGAACCTT,B,Homo_sapiens\\UCSC\\hg19\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Format != model.FormatV2 {
		t.Errorf("Format = %v, want v2", sheet.Format)
	}
	if len(sheet.Data) != 2 {
		t.Fatalf("len(Data) = %d, want 2", len(sheet.Data))
	}
	if sheet.Reads == nil || sheet.Reads.Read1Cycles != 151 || sheet.Reads.Read2Cycles != 151 {
		t.Errorf("Reads = %+v, want 151/151", sheet.Reads)
	}
	if sheet.Settings == nil {
		t.Fatal("Settings is nil")
	}
	if got, want := sheet.Settings.Extra["VariantFilterQualityCutoff"], "30"; got != want {
		t.Errorf("Extra[VariantFilterQualityCutoff] = %q, want %q", got, want)
	}
	if got, want := sheet.Settings.Extra["outputgenomevcf"], "FALSE"; got != want {
		t.Errorf("Extra[outputgenomevcf] = %q, want %q", got, want)
	}
	if len(sheet.Manifests) != 2 {
		t.Fatalf("len(Manifests) = %d, want 2", len(sheet.Manifests))
	}
	if got, want := sheet.Manifests[1].Filename, "TruSeqAmpliconManifest-2.txt"; got != want {
		t.Errorf("Manifests[1].Filename = %q, want %q", got, want)
	}
	// The unknown columns stay in Extra.
	if got, want := sheet.Data[0].Extra["GenomeFolder"], `Homo_sapiens\UCSC\hg19`; got != want {
		t.Errorf("Extra[GenomeFolder] = %q, want %q", got, want)
	}
}
