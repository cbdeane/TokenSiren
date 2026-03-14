{
  description = "Developer shell flake for clang";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
  };

  outputs = { self, nixpkgs }: {
    devShells.x86_64-linux.default = let
      pkgs = nixpkgs.legacyPackages.x86_64-linux;
      llvm = pkgs.llvmPackages;
      clangdWrapped = pkgs.writeShellScriptBin "clangd" ''
        exec ${pkgs.clang-tools}/bin/clangd \
          --query-driver=${pkgs.gcc}/bin/g++ \
          --gcc-toolchain=${pkgs.gcc} \
          "$@"
      '';
    in
      pkgs.mkShell {
        packages = with pkgs; [
          clangdWrapped
          clang
          clang-tools
          llvm.clang-unwrapped
          gcc
          cmake ninja
          bear
          pkg-config
	  go
          linuxHeaders
          libbpf
        ];
        shellHook = ''
          export CC=${pkgs.gcc}/bin/gcc
          export CXX=${pkgs.gcc}/bin/g++
          export BPF_CLANG=${llvm.clang-unwrapped}/bin/clang
          export BPF_INCLUDE=${pkgs.linuxHeaders}/include
          export BPF_INCLUDES="-I${pkgs.linuxHeaders}/include -I${pkgs.libbpf}/include -I$PWD/bpf"
          '';
      };
  };
}
