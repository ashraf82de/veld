Write `solution.veld` implementing a turnstile:

    pub type State
      case Locked
      case Unlocked(coins: Int)
      case Broken
    end type

    pub type Event
      case Coin
      case Push
      case Kick
    end type

    pub fn step(state: State, event: Event) -> State
    pub fn run(events: List[Event]) -> State

Rules for `step`:

- `Coin` unlocks a locked turnstile with 1 coin; on an unlocked one it adds a coin.
- `Push` locks an unlocked turnstile again; on a locked one nothing happens.
- `Kick` breaks it, whatever state it is in. A broken turnstile ignores everything.

`run` starts from `Locked` and applies the events in order.
