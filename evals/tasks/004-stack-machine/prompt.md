Write `solution.veld` defining a stack machine:

    pub type Instr
      case Push(value: Int)
      case Add
      case Mul
      case Dup
      case Pop
    end type

    pub fn run(program: List[Instr]) -> Result[List[Int], Str]

The stack starts empty and is returned top-last (the last pushed value is the
last element). `Add`/`Mul` pop two values and push the result; `Dup` pushes a
copy of the top; `Pop` removes the top. Any instruction that needs more values
than are on the stack returns `Err("stack underflow")`.
