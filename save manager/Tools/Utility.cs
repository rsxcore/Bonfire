namespace SoulsSaveManager.Tools
{
    internal static class Utility
    {
        // Prints the saved backups and returns how many console lines were written.
        internal static int GetSaveFolderList(string gameCustomBackupFolder, string defaultBackupFolderName)
        {
            var directories = new DirectoryInfo(gameCustomBackupFolder).GetDirectories()
                .Where(x => x.Name != defaultBackupFolderName)
                .ToArray();

            if (directories.Length == 0)
            {
                Console.WriteLine(" [!]> No saves yet");
                return 1;
            }

            int visualLineLength = directories.Max(x => x.Name.Length) + 10;
            Console.WriteLine($" ┌{new string('─', visualLineLength)}┐");

            for (int i = 0; i < directories.Length; i++)
            {
                string line = $"[{i + 1}]> {directories[i].Name}";
                Console.WriteLine($" │{line}{new string(' ', visualLineLength - line.Length)}│");
            }

            Console.WriteLine($" └{new string('─', visualLineLength)}┘");
            return directories.Length + 2;
        }

        // A save name must be a single folder name: no path separators, no "." or "..".
        internal static bool IsValidFolderName(string? name)
        {
            return !string.IsNullOrWhiteSpace(name)
                && name != "."
                && name != ".."
                && name.IndexOfAny(Path.GetInvalidFileNameChars()) < 0
                && Path.GetFileName(name) == name;
        }

        public static void CopyDirectory(string sourceDir, string destinationDir)
        {
            var dir = new DirectoryInfo(sourceDir);
            if (!dir.Exists)
                throw new DirectoryNotFoundException($"Source directory not found: {dir.FullName}");

            Directory.CreateDirectory(destinationDir);

            foreach (FileInfo file in dir.GetFiles())
                file.CopyTo(Path.Combine(destinationDir, file.Name), true);

            foreach (DirectoryInfo subDir in dir.GetDirectories())
                CopyDirectory(subDir.FullName, Path.Combine(destinationDir, subDir.Name));
        }

        // Replaces targetDir with a copy of sourceDir. The copy is made first, so if
        // anything fails the current contents of targetDir are left untouched.
        public static void ReplaceDirectory(string sourceDir, string targetDir)
        {
            string staging = targetDir.TrimEnd('/', '\\') + ".restore";
            string old = targetDir.TrimEnd('/', '\\') + ".old";

            if (Directory.Exists(staging))
                Directory.Delete(staging, true);
            if (Directory.Exists(old))
                Directory.Delete(old, true);

            CopyDirectory(sourceDir, staging);

            if (Directory.Exists(targetDir))
                Directory.Move(targetDir, old);

            try
            {
                Directory.Move(staging, targetDir);
            }
            catch
            {
                if (Directory.Exists(old))
                    Directory.Move(old, targetDir);
                throw;
            }

            if (Directory.Exists(old))
                Directory.Delete(old, true);
        }
    }
}
