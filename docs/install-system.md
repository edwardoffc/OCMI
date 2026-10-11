# Install OCMI: one result from either starting route

Full guided setup requires a local system copy, a client project bound to it,
private service memory and an initialized coordinator chat inside the project.
[BOOTSTRAP](../BOOTSTRAP.md) is the temporary installer;
[COORDINATOR](../COORDINATOR.md) is the permanent role.
Explanation is read-only; configuration needs selection and exact targets.

## From a repository URL

Give [the repository URL](https://github.com/edwardoffc/OCMI) to a browsing-capable chat.
The first response briefly explains OCMI and describes the folder/project/coordinator steps.
After selection, agree on a system folder (`00_OCMI` or `OCMI`) and obtain the full source.
The first chat supplies the user checklist below with the actual chosen paths.

## From a downloaded copy

Unpack the full repository into your chosen system folder.
Create a local OCMI project in the AI client, attach that exact folder, open a first chat
inside it and ask it to study OCMI and help with setup.
This is a temporary installer, not an initialized coordinator.
The installer verifies and reuses the download and this project; no duplicate clone or project is needed.
If you started in an ordinary chat instead, it gives the same binding checklist.
Downloading alone does not initialize the coordinator.

## Prepare core setup after selection

1. Agree on the system folder and intended client project name.
   Private service memory lives in `.local/coordinator` inside that system folder.
   Verify Git ignore/export exclusions and no tracked `.local` files before saving user values.
   No personal topic store is required. Preserve known existing core locations and records.
2. Verify source origin/revision/version. Prefer an identifiable tag/commit;
   record an unknown archive revision with its verification limit rather than inventing a pin.
   Do not execute remote scripts or install all profiles.
3. Choose one English/Russian template edition and prepare its coordinator service profile.
   Use [coordinator INSTALL](../templates/coordinator/INSTALL.md).
4. Fill private INDEX, BRIEF and HANDOFF with confirmed core facts.
   Record client project/binding and coordinator initialization separately as pending until supported
   by tools or the user's current confirmation. A proposed name is not a created project.
   Preserve any explicitly requested integration as deferred scope; read only minimal target instructions to classify an operational installation; defer code/test inspection and writes.
5. Fill all six tokens in [STARTUP](../templates/coordinator/STARTUP.md).
   The client-project field identifies the intended project and expected system-folder association.
   Save the prompt privately and give the user identical text plus the checklist below.
   Save user paths and filled prompts only in ignored local memory, never in tracked source or public packages.
6. Check files/links/access and report what is actually done:
   source ready → service/prompt ready → project/binding confirmed → coordinator initialized.
   Do not claim complete installation from files or chat creation alone.

## Checklist for the user

The installer must give these steps with the actual selected names and paths:

1. Create or select the local project OCMI in your AI client.
2. Attach the exact installed system folder to that project.
   Reuse an existing correctly bound project; do not create a second one.
3. Create a separate “OCMI Coordinator” chat inside this project.
4. Allow access to the exact private service directory if it is not already available.
   The standard service directory is inside the bound system folder; verify actual access.
5. Send the filled startup prompt supplied by the installer.
6. Wait for the coordinator to verify required files and confirm project/binding and startup.
   If those facts cannot be observed, give the requested current confirmation; do not treat a suggestion as evidence.
7. After recorded confirmation, you may delete/archive the temporary installer chat.

A client's equivalent folder-bound workspace is acceptable.
If the client cannot provide this association or required file access, setup remains pending;
a loose chat is not an equivalent completed installation. Explain the exact unsupported step.
Agents must not create/message/delete chats or change bindings without relevant authority.

## After confirmation

The coordinator explains practical usage, offers optional personal storage, folder organization
and work-project integration, and can resume a still-current explicitly deferred request.
Do not integrate work projects before confirmed core setup. Both component INSTALL files and the work-project routing rule enforce this entry procedure.
Existing installed projects keep
their own memory; this entry-route correction does not migrate or interrupt their routine work.

A new coordinator chat in the same bound project uses the privately saved STARTUP.md prompt
and current handoff. No installer history is required.
The coordinator ZIP is a service component; use the full repository for the local system.

## Updating and sharing

Preserve `.local` when updating the source or replacing a downloaded copy; never overwrite populated memory
with blank templates. Public source archives and component packages exclude `.local`.
Use Git source archives or exact template inventories for sharing, never a ZIP of the installed folder.
Do not read private memory during source development. Existing external coordinator locations remain valid
until an explicit migration request; do not move them or create competing records automatically.
