Write `solution.veld` defining:

    pub fn total(json_text: Str) -> Result[Float, Str]

The input is a JSON array of objects with numeric `price` and `qty` fields.
Return the sum of price * qty. Return `Err("invalid json")` if parsing fails
and `Err("bad item")` if the input is not an array or an item lacks either
numeric field.
