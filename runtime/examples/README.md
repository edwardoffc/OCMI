# Synthetic preview example

`init-request.json` is a deliberately fictional request for a new local preview
root. It is an operator/reference example, not a form to paste into a real
project. Replace every identity and the absolute `projectRoot` only after
choosing an explicit synthetic root.

The preview keeps its local operational state below `.ocmi/local` in that
chosen root; that is where its live SQLite database belongs. Do not copy that
database or its WAL into this examples directory, a preview archive, cloud
sync, coordinator memory, or another device. Keep credentials and real task
material out of these neutral examples too.

For a complete fictional cycle, ask an agent to use this selected root only, inspect `status`, bind
one task with distinct `worker` and `reviewer`, then report each response/revision through ready,
run, submit, review, integrate and resource close. Transfers carry only the exported JSON package
to a selected empty root; do not copy `.ocmi/local`, WAL files, logs or credentials.
