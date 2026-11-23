class Asciidraw < Formula
  desc "Draw ASCII mockups in the terminal with the mouse"
  homepage "https://github.com/wkoszek/asciidraw"
  url "https://github.com/wkoszek/asciidraw/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "28fdcff11d8b32b7fe5d9caac116e26cb5f9ef3bdab9e80e32496a01e6a7aecb"
  license "BSD-2-Clause"
  head "https://github.com/wkoszek/asciidraw.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w -X main.version=#{version}")
    man1.install "asciidraw.1"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/asciidraw --version")
    assert_match "usage: asciidraw", shell_output("#{bin}/asciidraw --help")
  end
end
