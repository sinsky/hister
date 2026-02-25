{
  lib,
  buildGoModule,
  buildNpmPackage,
  sqlite,
  pkg-config,
  histerRev ? "unknown",
}:
let
  version = (builtins.fromJSON (builtins.readFile ../ext/package.json)).version;

  frontend = buildNpmPackage {
    pname = "hister-frontend";
    inherit version;
    src = ../server/static/js;
    npmDepsHash = "sha256-BupgGlAhzanFyjv43terHsUUjmAxFniwMSBLFi8shC0=";
    dontNpmBuild = false;
    installPhase = ''
      runHook preInstall
      mkdir -p $out
      cp -r dist/* $out/
      runHook postInstall
    '';
  };
in
buildGoModule (finalAttrs: {
  pname = "hister";
  inherit version;

  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../go.mod
      ../go.sum
      ../hister.go
      ../server
      ../config
      ../ui
    ];
  };

  vendorHash = "sha256-KEuZ+jKG3fMYymZr9fvwlTzLFVcYfUAofe8DOIqHUDY=";
  proxyVendor = true;

  nativeBuildInputs = [ pkg-config ];
  buildInputs = [ sqlite ];

  tags = [ "libsqlite3" ];

  preBuild = ''
    mkdir -p server/static/js/dist
    cp -r ${frontend}/* server/static/js/dist/
  '';

  ldflags = [
    "-s"
    "-w"
    "-X main.version=${finalAttrs.version}"
    "-X main.commit=${histerRev}"
  ];

  subPackages = [ "." ];

  passthru = {
    inherit frontend;
  };

  meta = {
    description = "Web history on steroids - blazing fast, content-based search for visited websites";
    homepage = "https://github.com/asciimoo/hister";
    license = lib.licenses.agpl3Plus;
    maintainers = [ lib.maintainers.FlameFlag ];
    mainProgram = "hister";
    platforms = lib.platforms.unix;
  };
})
