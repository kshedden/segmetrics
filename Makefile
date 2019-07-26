collate2010: FORCE
	echo -n "cousub,tract,blockgroup" | rush 'go run collate.go -sumlevel={} -year=2010' -D ","

collate2000: FORCE
	echo -n "cousub,tract,blockgroup" | rush 'go run collate.go -sumlevel={} -year=2000' -D ","

cousub_metrics_2010: FORCE
	go run cmds.go metrics cousub 2010 "25000,45000,65000" | rush {}

cousub_metrics_2000: FORCE
	go run cmds.go metrics cousub 2000 "25000,45000,65000" | rush {}

tract_metrics_2010: FORCE
	go run cmds.go metrics tract 2010 "25000,45000,65000" | rush {}

tract_metrics_2000: FORCE
	go run cmds.go metrics tract 2000 "25000,45000,65000" | rush {}

blockgroup_metrics_2010: FORCE
	go run cmds.go metrics blockgroup 2010 "25000,45000,65000" | rush {}

blockgroup_metrics_2000: FORCE
	go run cmds.go metrics blockgroup 2000 "25000,45000,65000" | rush {}

metrics_2010: cousub_metrics_2010 tract_metrics_2010 blockgroup_metrics_2010

metrics_2000: cousub_metrics_2000 tract_metrics_2000 blockgroup_metrics_2000

cousub_normalize_2010: FORCE
	go run cmds.go normalize cousub 2010 "25000,45000,65000" | rush {}

tract_normalize_2010: FORCE
	go run cmds.go normalize tract 2010 "25000,45000,65000" | rush {}

blockgroup_normalize_2010: FORCE
	go run cmds.go normalize blockgroup 2010 "25000,45000,65000" | rush {}

normalize_2010: cousub_normalize_2010 tract_normalize_2010 blockgroup_normalize_2010

normalize_2000: cousub_normalize_2000 tract_normalize_2000 blockgroup_normalize_2000

cousub_normalize_2000: FORCE
	go run cmds.go normalize cousub 2000 "25000,45000,65000" | rush {}

tract_normalize_2000: FORCE
	go run cmds.go normalize tract 2000 "25000,45000,65000" | rush {}

blockgroup_normalize_2000: FORCE
	go run cmds.go normalize blockgroup 2000 "25000,45000,65000" | rush {}

gencsv_2010: FORCE
	go run cmds.go gencsv cousub 2010 "25000,45000,65000" | rush {}
	go run cmds.go gencsv tract 2010 "25000,45000,65000" | rush {}
	go run cmds.go gencsv blockgroup 2010 "25000,45000,65000" | rush {}

gencsv_2000: FORCE
	go run cmds.go gencsv cousub 2000 "25000,45000,65000" | rush {}
	go run cmds.go gencsv tract 2000 "25000,45000,65000" | rush {}
	go run cmds.go gencsv blockgroup 2000 "25000,45000,65000" | rush {}

upload_2010: FORCE
	go run cmds.go upload cousub 2010 "25000,45000,65000" | rush {} "remote:SegregationMetrics"
	go run cmds.go upload tract 2010 "25000,45000,65000" | rush {} "remote:SegregationMetrics"
	go run cmds.go upload blockgroup 2010 "25000,45000,65000" | rush {} "remote:SegregationMetrics"
	rclone copy segregation_2010.pdf "remote:SegregationMetrics"
	rclone copy /dsi/stage/stage/cscar-census/redistricting-data/docs/pl94-171-2010.pdf "remote:SegregationMetrics/docs"

upload_2000: FORCE
	go run cmds.go upload cousub 2000 "25000,45000,65000" | rush {} "remote:SegregationMetrics"
	go run cmds.go upload tract 2000 "25000,45000,65000" | rush {} "remote:SegregationMetrics"
	go run cmds.go upload blockgroup 2000 "25000,45000,65000" | rush {} "remote:SegregationMetrics"
	rclone copy segregation_2000.pdf "remote:SegregationMetrics"
	rclone copy /dsi/stage/stage/cscar-census/redistricting-data/docs/pl94-171-2000.pdf "remote:SegregationMetrics/docs"

FORCE:
