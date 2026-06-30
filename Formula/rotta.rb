class Rotta < Formula
  desc "Contract-driven development orchestrator for AI coding agents"
  homepage "https://github.com/Syfra3/Rotta"
  version "0.0.0"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/Syfra3/Rotta/releases/download/rotta-v#{version}/rotta-#{version}-darwin-arm64.tar.gz"
      sha256 "TODO_DARWIN_ARM64"
    else
      url "https://github.com/Syfra3/Rotta/releases/download/rotta-v#{version}/rotta-#{version}-darwin-amd64.tar.gz"
      sha256 "TODO_DARWIN_AMD64"
    end
  end

  on_linux do
    if Hardware::CPU.intel?
      url "https://github.com/Syfra3/Rotta/releases/download/rotta-v#{version}/rotta-#{version}-linux-amd64.tar.gz"
      sha256 "TODO_LINUX_AMD64"
    else
      url "https://github.com/Syfra3/Rotta/releases/download/rotta-v#{version}/rotta-#{version}-linux-arm64.tar.gz"
      sha256 "TODO_LINUX_ARM64"
    end
  end

  def install
    bin.install "rotta"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/rotta --version")
  end
end
