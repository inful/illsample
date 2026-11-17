# Illumina sample sheets

The sample sheets that Illumina publishes, used as test fixtures. They
are the reference for the formats the parser has to read, so every
change to the parser is checked against them.

| File | Source |
|------|--------|
| `SampleSheet_v2_template.csv` | <https://help.bioinsight.illumina.com/run-set-up/overview> (Run Set Up → Sample Sheet) |
| `SampleSheet_NovaSeq6000_Example.csv` | <https://emea.support.illumina.com/downloads/novaseq-6000-example-sample-sheet--s4-hcp-illumina-udi-lt.html> |
| `SampleSheet_TEMPLATE_*.csv` | <https://support.illumina.com/downloads/illumina-5-base-dna-prep-sample-sheet-templates.html> |
| `Sample_Sheet_Template_TSO_500*.csv` | <https://help.tso500software.illumina.com/dragen-tso-500-guides/dragen-tso-500-v2.6/run-planning/sample-sheet-templates> (DRAGEN TSO 500) |

The files are Illumina's, downloaded from the pages above. They are
kept as they are published, so that the parser is tested against the
real thing.

The TSO 500 templates are eleven sheets: the standard, XP and
high-throughput workflows, for both the i5-reverse and the
i5-forward index orientation, and for the local-launch and
auto-launch dialects. They use the padded-comma layout, name the
data section `[BCLConvert_Data]`, and carry a `[Sequencing_Settings]`
section that no other sheet has.
