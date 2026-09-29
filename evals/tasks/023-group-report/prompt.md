Write `solution.veld` with

    pub record Sale
      region: Str
      product: Str
      units: Int
      price: Float
    end record

    pub fn report(sales: List[Sale]) -> List[Str]

Produce a text report: one line per region, sorted by region name, formatted

    <region>: <total units> units, revenue <revenue with 2 decimals>, top: <product>

where revenue is the sum of `units * price` over the region's sales, and `top` is
the product with the most units in that region (the alphabetically first one on a
tie). Empty input gives `[]`.
