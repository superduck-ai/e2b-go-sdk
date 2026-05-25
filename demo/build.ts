import tailwind from "bun-plugin-tailwind";

await Bun.build({
  entrypoints: ["./frontend.tsx"],
  outdir: "./dist",
  splitting: true,
  naming: "[name].[ext]",
  plugins: [tailwind],
});

await Bun.build({
  entrypoints: ["./admin.tsx"],
  outdir: "./dist",
  splitting: true,
  naming: "[name].[ext]",
  plugins: [tailwind],
});
