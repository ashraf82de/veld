Write `solution.veld` implementing a small bank:

    pub record Account
      id: Str
      balance: Int
    end record

    pub type Tx
      case Deposit(id: Str, amount: Int)
      case Withdraw(id: Str, amount: Int)
      case Transfer(from: Str, to: Str, amount: Int)
    end type

    pub fn apply(accounts: Map[Str, Account], tx: Tx) -> Result[Map[Str, Account], Str]
    pub fn apply_all(accounts: Map[Str, Account], txs: List[Tx]) -> Result[Map[Str, Account], Str]

Rules for `apply`: amounts must be positive (`Err("invalid amount")`); an unknown
account is `Err("unknown account: <id>")` (for a transfer check `from` first, then
`to`); a withdrawal or transfer that would make a balance negative is
`Err("insufficient funds")`; a transfer to the same account is `Err("same account")`.
On error the input is unchanged. `apply_all` applies the transactions in order and
stops at the first error, returning it.
