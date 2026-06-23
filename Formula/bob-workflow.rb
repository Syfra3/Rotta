class BobWorkflow < Formula
  desc "Contract-driven development orchestrator with Uncle Bob workflow"
  homepage "https://github.com/Syfra3/bob-workflow"
  version "0.0.0"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/Syfra3/bob-workflow/releases/download/v#{version}/bob-workflow-#{version}-darwin-arm64.tar.gz"
      sha256 "TODO_DARWIN_ARM64"
    else
      url "https://github.com/Syfra3/bob-workflow/releases/download/v#{version}/bob-workflow-#{version}-darwin-amd64.tar.gz"
      sha256 "TODO_DARWIN_AMD64"
    end
  end

  on_linux do
    if Hardware::CPU.intel?
      url "https://github.com/Syfra3/bob-workflow/releases/download/v#{version}/bob-workflow-#{version}-linux-amd64.tar.gz"
      sha256 "TODO_LINUX_AMD64"
    else
      url "https://github.com/Syfra3/bob-workflow/releases/download/v#{version}/bob-workflow-#{version}-linux-arm64.tar.gz"
      sha256 "TODO_LINUX_ARM64"
    end
  end

  def install
    bin.install "uncle-bob"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/uncle-bob --version")
  end
end
