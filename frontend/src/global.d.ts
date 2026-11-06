export {};

declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          ListProjects: () => Promise<any[]>;
          CreateProject: (title: string, premise: string) => Promise<any>;
          RunMock: (id: string) => Promise<any>;
          ChooseImportFolder: () => Promise<any[]>;
          RecoverExistingAssets: (configPath: string) => Promise<any[]>;
          TestAbocadoConnection: (configPath: string) => Promise<any>;
          BeginAbocadoAuth: (configPath: string) => Promise<string>;
        };
      };
    };
  }
}

