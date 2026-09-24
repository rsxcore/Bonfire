using SoulsSaveManager.Tools;
using System.Drawing;

namespace SoulsSaveManager
{
    internal class SaveManager
    {
        #region game list
        internal static string[] SoulsGameList =
        [
            "Dark Souls: Prepare To Die Edition",
            "Dark Souls: Remastered",
            "Dark Souls II",
            "Dark Souls III",
            "Sekiro: Shadows Die Twice",
            "Elden Ring",
            "Bloodborne"
        ];
        #endregion

        private static readonly Color BackupColor = Color.FromArgb(255, 198, 107, 255);
        private static readonly Color LoadColor = Color.FromArgb(255, 255, 110, 144);

        internal SaveManager(string gameName, string savePath, string saveFolderName, string customFolderName)
        {
            GameName = gameName;
            _savePath = savePath;
            _saveFolderName = saveFolderName;
            _customFolderName = customFolderName;
        }

        internal string GameName { get; private set; }
        private string _savePath, _saveFolderName, _customFolderName;

        private bool isExit = false;

        private string GameBackupPath => WorkingPaths.DesktopPath + $"/{_customFolderName}";

        // The default backup lives in its own folder next to the named saves.
        private string DefaultBackupPath => GameBackupPath + $"/{_saveFolderName}";

        private string NamedBackupPath(string name) => GameBackupPath + $"/{name}/{_saveFolderName}";

        private static string Now => DateTime.Now.ToString("HH:mm:ss");

        internal void Invoke()
        {
            ConsoleUtility.WriteBox(["!> To change the game press 'F1'"], GameName);
            ConsoleUtility.WriteBox([
                "$> [1]> Backup save to default folder",
                "$> [2]> Load save from default folder",
                "$> [3]> Backup save to another directory",
                "$> [4]> Load save from another directory",
                "$> [5]> Delete save",
            ]);
            Console.WriteLine();

            while (!isExit)
            {
                SelectBackupFunctions();
            }

            Console.Clear();
        }

        private void SelectBackupFunctions()
        {
            _ = Directory.CreateDirectory(WorkingPaths.DesktopPath);

            ConsoleKeyInfo keyInfo = Console.ReadKey(true);

            // A failed copy used to be silently ignored and reported as a success.
            try
            {
                switch (keyInfo.Key)
                {
                    case ConsoleKey.F1:
                        isExit = true;
                        break;

                    case ConsoleKey.D1:
                        MakeBackupToDefaultFolder();
                        break;

                    case ConsoleKey.D2:
                        LoadBackupFromDefaultFolder();
                        break;

                    case ConsoleKey.D3:
                        MakeBackupToCustomFolder();
                        break;

                    case ConsoleKey.D4:
                        LoadBackupFromCustomFolder();
                        break;

                    case ConsoleKey.D5:
                        DeleteSave();
                        break;
                }
            }
            catch (Exception ex)
            {
                Colorful.Console.WriteLine($" [!]> Failed: {ex.Message}", Color.Red);
            }
        }

        private void MakeBackupToDefaultFolder()
        {
            ConsoleUtility.RemoveLine();

            if (!Directory.Exists(_savePath))
            {
                Colorful.Console.WriteLine(" [!]> Save directory doesn't exist", Color.Red);
                return;
            }

            Utility.ReplaceDirectory(_savePath, DefaultBackupPath);

            Colorful.Console.WriteLine($" [!]> Default save successfully backed up at {Now}", BackupColor);
        }

        private void LoadBackupFromDefaultFolder()
        {
            ConsoleUtility.RemoveLine();

            if (!Directory.Exists(DefaultBackupPath))
            {
                Colorful.Console.WriteLine(" [!]> There is no default backup yet", Color.Red);
                return;
            }

            Utility.ReplaceDirectory(DefaultBackupPath, _savePath);

            Colorful.Console.WriteLine($" [!]> Default save successfully loaded at {Now}", LoadColor);
        }

        private void MakeBackupToCustomFolder()
        {
            ConsoleUtility.RemoveLine();

            if (!Directory.Exists(_savePath))
            {
                Colorful.Console.WriteLine(" [!]> Save directory doesn't exist", Color.Red);
                return;
            }

            Colorful.Console.Write(" [?]> New folder name >>> ", BackupColor);
            string? name = Console.ReadLine();
            ConsoleUtility.RemoveLine();

            if (!Utility.IsValidFolderName(name) || name == _saveFolderName)
            {
                Colorful.Console.WriteLine($" [!]> \"{name}\" is not a valid folder name", Color.Red);
                return;
            }

            Utility.ReplaceDirectory(_savePath, NamedBackupPath(name!));

            Colorful.Console.WriteLine($" [!]> Save successfully backed up to \"{name}\" at {Now}", BackupColor);
        }

        // Shows the named saves and asks for one. Returns null (after printing why) if there's nothing to pick.
        private string? PickNamedSave(Color promptColor)
        {
            ConsoleUtility.RemoveLine();

            if (!Directory.Exists(GameBackupPath))
            {
                Colorful.Console.WriteLine($" [!]> {_customFolderName} doesn't exist", Color.Red);
                return null;
            }

            int listLines = Utility.GetSaveFolderList(GameBackupPath, _saveFolderName);

            Colorful.Console.Write(" [?]> Folder name >>> ", promptColor);
            string? name = Console.ReadLine();
            ConsoleUtility.RemoveLines(listLines + 1);

            // Only plain folder names are accepted, so input like ".." can't reach outside the backups.
            if (!Utility.IsValidFolderName(name) || name == _saveFolderName || !Directory.Exists(GameBackupPath + $"/{name}"))
            {
                Colorful.Console.WriteLine($" [!]> \"{name}\" folder doesn't exist", Color.Red);
                return null;
            }

            return name;
        }

        private void LoadBackupFromCustomFolder()
        {
            string? name = PickNamedSave(LoadColor);
            if (name == null)
                return;

            Utility.ReplaceDirectory(NamedBackupPath(name), _savePath);

            Colorful.Console.WriteLine($" [!]> \"{name}\" save successfully loaded at {Now}", LoadColor);
        }

        private void DeleteSave()
        {
            string? name = PickNamedSave(LoadColor);
            if (name == null)
                return;

            Directory.Delete(GameBackupPath + $"/{name}", true);

            Colorful.Console.WriteLine($" [!]> \"{name}\" save successfully deleted at {Now}", LoadColor);
        }
    }
}
