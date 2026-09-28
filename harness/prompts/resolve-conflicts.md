## Task: resolve merge conflicts

The harness merged `{{BASE}}` into `{{BRANCH}}` and the merge stopped on conflicts. Run
`git status` to see them. Resolve every conflict so that both the feature on this branch
and the new work on `{{BASE}}` keep working, then `git add` the files. Don't commit and
don't abort the merge: the harness concludes it for you.

In `{{OUT}}/summary.md`, use a title like `chore: merge main into this branch`, and
describe how you resolved each conflict under Changes.

{{TASK}}
