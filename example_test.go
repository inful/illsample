package illsample_test

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/inful/illsample"
)

// This example shows the typical parse-verify-write loop.
func ExampleParse() {
	input := `Sample_ID,Sample_Name,index,Sample_Project
sample_01,sample_one,ATCGATCG,RNAseq
sample_02,sample_two,GCTAGCTA,RNAseq
`
	sheet, err := illsample.Parse(strings.NewReader(input))
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("format=%s rows=%d\n", sheet.Format, len(sheet.Data))
	// Output: format=v1 rows=2
}

// This example shows how to detect specific verification issues.
func ExampleVerify() {
	sheet := &illsample.SampleSheet{
		Format: illsample.FormatV1,
		Header: []string{"Sample_ID", "Sample_Name", "index", "Sample_Project"},
		Data: []illsample.Sample{
			{SampleID: "id1", SampleName: "name1", Index: "ATCG", SampleProject: "P1"},
			{SampleID: "id1", SampleName: "name2", Index: "GCTA", SampleProject: "P1"},
		},
	}
	result := illsample.Verify(sheet)
	fmt.Printf("errors=%d warnings=%d\n", len(result.Errors()), len(result.Warnings()))
	for _, iss := range result.Errors() {
		fmt.Printf("  %s: %s\n", iss.Code, iss.Message)
	}
	// Output:
	// errors=1 warnings=0
	//   E_V1_DUPLICATE_SAMPLE_ID: Sample_ID "id1" is also used in row 0
}

// This example shows how to convert a sheet from one format to the
// other in memory.
func ExampleWrite() {
	sheet := &illsample.SampleSheet{
		Format: illsample.FormatV1,
		Header: []string{"Sample_ID", "Sample_Name", "index", "Sample_Project"},
		Data: []illsample.Sample{
			{SampleID: "id1", SampleName: "name1", Index: "ATCG", SampleProject: "P1"},
		},
	}
	sheet.Format = illsample.FormatV2
	var buf bytes.Buffer
	if err := illsample.Write(&buf, sheet); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Print(buf.String())
	// Output:
	// [Data]
	// Sample_ID,Sample_Name,Sample_Plate,Sample_Well,I7_Index_ID,index,I5_Index_ID,index2,Sample_Project,Description
	// id1,name1,,,,ATCG,,,P1,
}

// This example shows how to gate a build pipeline on validation
// errors using VerifyWithOptions to treat warnings as errors.
func ExampleVerifyWithOptions() {
	sheet := &illsample.SampleSheet{
		Format: illsample.FormatV2,
		Header: []string{"Sample_ID", "Sample_Name", "Sample_Plate", "Sample_Well", "I7_Index_ID", "index", "I5_Index_ID", "index2", "Sample_Project", "Description"},
		Reads:  &illsample.Reads{Read1Cycles: 151, Index1Cycles: 8},
		// No Settings -> W_V2_NO_ADAPTER warning.
		Data: []illsample.Sample{
			{SampleID: "id1", SampleName: "name1", Index: "ATCGATCG", SampleProject: "P1"},
		},
	}
	defaultResult := illsample.Verify(sheet)
	strict := illsample.VerifyWithOptions(sheet, illsample.VerifyOptions{PromoteWarningsToErrors: true})
	fmt.Printf("default: errors=%d warnings=%d\n", len(defaultResult.Errors()), len(defaultResult.Warnings()))
	fmt.Printf("strict:  errors=%d warnings=%d\n", len(strict.Errors()), len(strict.Warnings()))
	// Output:
	// default: errors=0 warnings=1
	// strict:  errors=1 warnings=1
}
