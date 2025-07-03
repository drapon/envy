# typed: false
# frozen_string_literal: true

# Homebrew formula for envy
class Envy < Formula
  desc "Environment variable sync tool between local files and AWS Parameter Store/Secrets Manager"
  homepage "https://github.com/drapon/envy"
  version "VERSION_PLACEHOLDER"
  license "MIT"

  # macOS binaries
  on_macos do
    if Hardware::CPU.intel?
      url "https://github.com/drapon/envy/releases/download/vVERSION_PLACEHOLDER/envy_Darwin_x86_64.tar.gz"
      sha256 "SHA256_DARWIN_AMD64_PLACEHOLDER"
    end

    if Hardware::CPU.arm?
      url "https://github.com/drapon/envy/releases/download/vVERSION_PLACEHOLDER/envy_Darwin_arm64.tar.gz"
      sha256 "SHA256_DARWIN_ARM64_PLACEHOLDER"
    end
  end

  # Linux binaries
  on_linux do
    if Hardware::CPU.intel?
      url "https://github.com/drapon/envy/releases/download/vVERSION_PLACEHOLDER/envy_Linux_x86_64.tar.gz"
      sha256 "SHA256_LINUX_AMD64_PLACEHOLDER"
    end

    if Hardware::CPU.arm? && Hardware::CPU.is_64_bit?
      url "https://github.com/drapon/envy/releases/download/vVERSION_PLACEHOLDER/envy_Linux_arm64.tar.gz"
      sha256 "SHA256_LINUX_ARM64_PLACEHOLDER"
    end
  end

  # Git dependency (optional)
  depends_on "git" => :optional

  def install
    bin.install "envy"

    # Generate and install shell completion files
    generate_completions_from_executable(bin/"envy", "completion")

    # Install man page (if exists)
    man1.install "envy.1" if File.exist?("envy.1")
  end

  def post_install
    # Create configuration directory
    (etc/"envy").mkpath
  end

  def caveats
    <<~EOS
      envy has been installed!

      Getting started:
        1. Initialize in your project directory:
           $ envy init

        2. Edit .envyrc file to customize settings

        3. Sync environment variables to AWS:
           $ envy push

      For detailed documentation, see:
        https://github.com/drapon/envy

      AWS credentials setup is required:
        - AWS CLI configuration
        - Or environment variables (AWS_PROFILE, AWS_REGION, etc.)
        - Or IAM role (EC2/ECS environment)

      Sample configuration file:
        #{etc}/envy/envyrc.example
    EOS
  end

  test do
    # Version display test
    assert_match version.to_s, shell_output("#{bin}/envy version")

    # Help display test
    assert_match "Environment variable sync tool", shell_output("#{bin}/envy --help")

    # Configuration file generation test (dry-run)
    ENV["ENVY_DRY_RUN"] = "true"
    system bin/"envy", "init"
    assert_predicate testpath/".envyrc", :exist?
  end

  service do
    run [opt_bin/"envy", "watch"]
    keep_alive true
    working_dir var/"envy"
    log_path var/"log/envy.log"
    error_log_path var/"log/envy.error.log"
    environment_variables ENVY_CONFIG: etc/"envy/envyrc"
  end
end