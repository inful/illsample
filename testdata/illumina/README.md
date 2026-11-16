# Illumina sample sheets

The sample sheets that Illumina publishes, used as test fixtures. They
are the reference for the formats the parser has to read, so every
change to the parser is checked against them.

| File | Source |
|------|--------|
| `SampleSheet_v2_template.csv` | <https://help.bioinsight.illumina.com/run-set-up/overview> (Run Set Up → Sample Sheet) |
| `SampleSheet_NovaSeq6000_Example.csv` | <https://emea.support.illumina.com/downloads/novaseq-6000-example-sample-sheet--s4-hcp-illumina-udi-lt.html> |
| `SampleSheet_TEMPLATE_*.csv` | <https://support.illumina.com/downloads/illumina-5-base-dna-prep-sample-sheet-templates.html> |

The files are Illumina's, downloaded from the pages above. They are
kept as they are published, so that the parser is tested against the
real thing.
