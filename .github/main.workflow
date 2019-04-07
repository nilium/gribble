workflow "Test" {
  on = "push"
  resolves = [
    "Print Function Coverage",
    "Build Server",
  ]
}

action "Verify Modules" {
  uses = "./.github/golang"
  args = ["mod", "verify"]
}

action "Go Test" {
  needs = ["Verify Modules"]
  uses  = "./.github/golang"
  args  = ["test", "-v", "-coverprofile=cover.out", "-coverpkg=go.spiff.io/gribble/...", "./..."]
}

action "Print Function Coverage" {
  needs = ["Go Test"]
  uses  = "./.github/golang"
  args  = ["tool", "cover", "-func", "cover.out"]
}

action "Build Server" {
  needs = ["Go Test"]
  uses  = "./.github/golang"
  args  = ["build", "-v", "./cmd/gribblesv"]
}
