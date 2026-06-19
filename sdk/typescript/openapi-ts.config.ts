export default {
  input: "../../openapi/sdk-openapi.json",
  output: {
    module: {
      extension: ".js",
    },
    path: "src/rest",
  },
  plugins: [
    {
      includeInEntry: true,
      name: "@hey-api/client-fetch",
    },
  ],
};
