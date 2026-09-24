namespace SoulsSaveManager
{
    internal static class WorkingPaths
    {
        internal static string AppDataPath = Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData);
        internal static string DesktopPath = Environment.GetFolderPath(Environment.SpecialFolder.Desktop) + "/backup";
        internal static string DarkSoulsPreparePath = Environment.GetFolderPath(Environment.SpecialFolder.MyDocuments) + "/NBGI/DarkSouls";
        internal static string DarkSoulsRemasteredPath = Environment.GetFolderPath(Environment.SpecialFolder.MyDocuments) + "/NBGI/DARK SOULS REMASTERED";
        internal static string DarkSoulsIIPath = AppDataPath + "/DarkSoulsII";
        internal static string DarkSoulsIIIPath = AppDataPath + "/DarkSoulsIII";
        internal static string SekiroPath = AppDataPath + "/Sekiro";
        internal static string EldenRingPath = AppDataPath + "/EldenRing";
        internal static string BloodbornePath = "";

        // Stored next to the exe, so it doesn't depend on the folder the app was started from.
        internal static string ConfigPath = Path.Combine(AppContext.BaseDirectory, "config.cfg");

        // Bloodborne title IDs: GOTY US, US, EU, GOTY EU, Asia.
        internal static string[] BloodborneTitleIds = ["CUSA03173", "CUSA00900", "CUSA00207", "CUSA03023", "CUSA01363"];

        // Finds the Bloodborne save inside a shadPS4 folder, whichever region the game is.
        internal static string? FindBloodborneSave(string emulatorPath)
        {
            string saveDataPath = Path.Combine(emulatorPath, "user", "savedata");
            if (!Directory.Exists(saveDataPath))
                return null;

            foreach (string userPath in Directory.GetDirectories(saveDataPath))
            {
                foreach (string titleId in BloodborneTitleIds)
                {
                    string candidate = Path.Combine(userPath, titleId);
                    if (Directory.Exists(candidate))
                        return candidate;
                }
            }

            return null;
        }
    }
}
