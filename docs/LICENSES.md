# Issuing ATO licenses

How Clause Eight sets up signing keys and issues, renews and replaces customer licenses. How licenses work and
why: [ADR 15](adr/0015-licensing.md). What a customer does with one: README, "Your license".

The private signing key is the only secret in this process. Anyone who has it can make licenses that every
released ATO accepts. It never goes into a repository, a CI variable, a chat, an email or a ticket.

## Once: the signing key

1. Build the issuing tool (it is not part of a release):

   ```sh
   go build -o ~/bin/ato-license ./cmd/ato-license
   ```

2. Make the key pair in a folder outside every repository. Name the key after the year and month, so a later
   key sorts after it:

   ```sh
   ato-license keygen -id 2026-11 -out ~/.config/c8-license
   ```

   This writes `2026-11.key` (private, readable only by you) and `2026-11.pub` (public). It refuses to
   overwrite an existing key.

3. Back up `2026-11.key` at once, in a second place you control: a password manager entry with the file
   attached, or an encrypted USB key kept offline. If the only copy is lost, no new license can be issued with
   that key, and every customer needs a new license from a new key.

4. Put the public key in ATO and release:

   ```sh
   cp ~/.config/c8-license/2026-11.pub internal/license/keys/
   git add internal/license/keys/2026-11.pub    # the .pub only; .gitignore refuses *.key
   ```

   Open a pull request, merge it, and tag a release. Builds without the key reject licenses it signs, so
   customers need this release or a later one.

## Each customer: issuing a license

Issue one license per organisation and contract term:

```sh
ato-license issue \
  -key ~/.config/c8-license/2026-11.key \
  -org "Example Org SRL" \
  -plan privacy \
  -seats 25 \
  -expires 2027-11-30 \
  -out ~/Documents/c8-licenses/example-org-2026.license \
  -ledger ~/Documents/c8-licenses/issued.csv
```

- `-org` is the customer's legal name as on the contract. Never put a person's name or email here: the license
  travels into the customer's CI logs and config files.
- `-plan` is the plan name from the price list. Use the same few names every time (for example `privacy`,
  `team`); a later release may switch features on and off by plan.
- `-seats` is the number on the contract. ATO shows it but does not count users.
- `-expires` is the last day the license is valid; ATO keeps working for 30 more days after it. Set it to the
  end of the paid term.
- `-features` (optional) is a comma-separated list for anything sold on top of the plan.
- `-issued` defaults to today; `-id` defaults to `ato-<issued>-<random>`.
- `-ledger` appends the terms to a CSV, your record of every license issued. Keep it with the contracts, not in
  a repository.

The tool warns when the key is not yet built into ATO. Check the file before sending it:

```sh
ato-license inspect ~/Documents/c8-licenses/example-org-2026.license
```

It should say `valid` with the right organisation, plan and dates. Before the public key is released, check
with `-pub ~/.config/c8-license/2026-11.pub` instead.

## Sending it

Email the `.license` file to the customer's technical contact, with these steps:

> On each machine that runs ATO: `ato license install example-org-2026.license`, then `ato license` to check.
>
> In CI, store the license as a secret named `ATO_LICENSE`. That means a repository or organisation secret on
> GitHub, passed to the action as `license: ${{ secrets.ATO_LICENSE }}`; a masked, protected CI/CD variable on
> GitLab; or a secured variable on Bitbucket. The value is the line that starts with `ato-license-v1.`.

The license file is not a secret in the way the signing key is, but it is the customer's paid entitlement. Send
it only to the customer, and ask them to keep it out of public repositories.

## Renewals and changes

- **Renewal:** issue a new license with the new expiry date and send it. The customer installs it over the old
  one and updates the CI secret. Send it before the end date; the 30-day grace period covers a late renewal.
- **More seats or a different plan:** issue a new license with the new terms and the same end date.
- **Not renewing:** do nothing. After the end date and the 30-day grace, ATO keeps running as an evaluation and
  says so on every run. The contract, not the software, ends the right to use it.
- **A customer asks why ATO says it is unlicensed:** ask for the output of `ato license`. It names the file or
  variable it read and the reason: no license, a changed or damaged file, a key their build does not know (they
  need a newer release), or an expired term.

## Keys over time

- **Rotation:** once a year, make a new key (`-id 2027-11`), release its `.pub`, and issue new licenses with it.
  Keep the old `.pub` in `internal/license/keys/` so licenses signed with the old key keep working until they
  expire.
- **Leaked or lost private key:** make a new key, delete the old `.pub` from `internal/license/keys/`, release,
  and reissue every license the old key signed (the ledger lists them by key). Licenses cannot be revoked one by
  one: a leaked customer license stays valid until its end date, so keep terms to one year.

## What licensing does not do

- ATO never contacts a server, so it cannot see how many people use a license, and it cannot be switched off
  remotely.
- A determined user can patch the binary or turn back the clock. That is a breach of the contract, which is what
  enforcement rests on.
- Today a missing or expired license only adds a notice; ATO works in full. Limiting the evaluation is a change in
  `licenseNotice` (internal/cli/license.go), and the plan and features in every license are already there for it.
