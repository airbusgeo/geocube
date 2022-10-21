# Release notes

## 1.0.1

### Functionalities
- Consolidater: add option `--local-download` (default=`true`) to download datasets locally before starting the consolidation. Usually, it's faster to download first, but in some case, it's not (or
consume a lot of local storage)
- API: add GetRecords(List IDs)
- API: GetCube: add ResamplingAlg (override variable.ResamplingAlg)

### Bug fixes
- Cancel consolidation tasks took to much time (due to job being saved at every task)
- Update mod airbusgeo/cogger to fix a crash with overviews
- If a deletion task failed, the job must be in "DONEBUTUNTIDY" state
