Write `solution.veld` defining:

    pub record Item
      sku: Str
      qty: Int
    end record

    pub type Event
      case Received(sku: Str, qty: Int)
      case Shipped(sku: Str, qty: Int)
    end type

    pub fn apply_events(stock: List[Item], events: List[Event]) -> Result[List[Item], Str]

Apply events in order. `Received` adds quantity (creating the item at the end
of the list if new). `Shipped` removes quantity; if the item is missing or
would go negative, return `Err("insufficient <sku>")`. Items keep their order.
