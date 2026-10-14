---
title: "Boss Challenge: Build a Greeter"
xp: 150
difficulty: hard
boss: true
expected_tests:
  - TestGreetNamedPerson
  - TestGreetBlankName
---

# Goal

Build a complete greeting function that handles both names and blank input.

# Example

`Greet("Ada")` should return `Hello, Ada!`.

# What to do

Trim surrounding spaces, and use `stranger` when the name is blank.
