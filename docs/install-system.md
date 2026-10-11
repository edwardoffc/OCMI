# Install the system and hand over to a coordinator

Use [BOOTSTRAP](../BOOTSTRAP.md) for the first chat and
[COORDINATOR](../COORDINATOR.md) for the permanent chat.
The first chat explains briefly and offers installation; merely supplying a URL changes nothing.

## Install after selection

1. Agree on the system folder (`00_OCMI` or `OCMI` are suggested names) and a separate,
   private service directory such as `OCMI-local/coordinator`.
   State both targets before writing. No personal topic store is required.
   Preserve a known existing coordinator directory and its records.
2. Obtain a full local copy of [the repository](https://github.com/edwardoffc/OCMI).
   Prefer a verified tag/commit; an existing downloaded checkout is sufficient after verification.
   Record origin, revision and version. If an archive has no verifiable commit, record that limit;
   do not describe a moving branch or an unverified archive as pinned.
   Do not run remote setup scripts or create every profile.
3. Choose one template edition, English or Russian; communication language can differ.
   Read its coordinator INSTALL and prepare its private service memory.
   Use [coordinator INSTALL](../templates/coordinator/INSTALL.md);
   Russian instructions use the same stable paths under locales/ru.
4. Fill INDEX, BRIEF and HANDOFF with confirmed core locations, source identity,
   language and actual status. Optional actions remain not selected.
   Render [STARTUP](../templates/coordinator/STARTUP.md): replace every angle-bracket token.
   Save the rendered prompt in private STARTUP.md and give the identical text to the user.
   Missing authorization/paths must be resolved first; unknown revision may be explicitly
   recorded as unknown with its verification limit. Do not invent verification.
5. Check local links, actual read/write capabilities and restoration inputs.
   Report source installed / handover prepared separately from coordinator initialized.
   Never put user paths, installation records or filled prompts into the public source checkout.

## User actions in the AI client

Create a local project named OCMI and attach the installed system folder.
Inside it create a separate chat, for example “OCMI Coordinator”, and send the filled prompt.
If the client has no projects, a separate chat with access to the system folder is sufficient.

The prompt also identifies the private service directory. If the client denies access,
grant/attach that specific directory as an additional folder, or use its supported file-access
mechanism. Attaching the system alone does not guarantee access to a sibling directory.
No access to unrelated personal topics or work projects is needed for startup.

The coordinator checks the saved inputs and records confirmation before suggesting optional
personal storage, organization and project integration. Keep the installer chat until this succeeds.
After confirmation the user may delete/archive the installer chat; the agent does not do it automatically.

## Continue later

Open a new coordinator chat in the same project and send the saved private STARTUP.md prompt.
It restores the current handoff instead of relying on the old conversation.
A moved folder requires updating confirmed paths and client bindings; do not rewrite history silently.
Work projects and personal topics continue separately from their own role/entry/handoff.

The coordinator ZIP contains only the service profile. It does not replace the required
full local system copy in this guided installation flow.
Experienced users may still adopt a work/lightweight template directly without a central coordinator;
installed projects remain self-contained.
