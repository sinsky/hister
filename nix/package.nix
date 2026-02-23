{
  lib,
  buildGoModule,
  sqlite,
  histerRev ? "unknown",
}:
let
  packageJson = builtins.fromJSON (builtins.readFile ../ext/package.json);
in
buildGoModule (finalAttrs: {
  pname = "hister";
  version = packageJson.version;

  src = ../.;

  vendorHash = "sha256-BoM5CN57UuuxCaf1myyQxUyT2N1uGy+IjpCvDJmfvAU=";

  buildInputs = [ sqlite ];

  preBuild = ''
    export CGO_CFLAGS="-I${sqlite.dev}/include"
    export CGO_LDFLAGS="-L${sqlite.out}/lib -lsqlite3"
  '';

  ldflags = [
    "-s"
    "-w"
    "-X main.version=${finalAttrs.version}"
    "-X main.commit=${histerRev}"
  ];

  subPackages = [ "." ];

  doCheck = false;

  meta = {
    description = "Web history on steroids - blazing fast, content-based search for visited websites";
    homepage = "https://github.com/asciimoo/hister";
    license = lib.licenses.agpl3Plus;
    maintainers = [ lib.maintainers.FlameFlag ];
    mainProgram = "hister";
    platforms = lib.platforms.unix;
  };
})
