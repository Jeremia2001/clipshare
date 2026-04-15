import os
from pathlib import Path

class Config:
    """Application configuration management."""
    
    def __init__(self):
        self.api_url = os.getenv('CLIPSHARE_API_URL', 'http://localhost:8080')
        self.dev_mode = os.getenv('CLIPSHARE_DEV', 'false').lower() == 'true'
        
        # Paths
        self.app_dir = Path(__file__).parent.parent.absolute()
        self.ffmpeg_path = self.app_dir / 'ffmpeg' / 'ffmpeg'
        
        # Config storage
        self.config_dir = Path.home() / '.config' / 'clipshare'
        self.config_dir.mkdir(parents=True, exist_ok=True)
        
    def get_ffmpeg_cmd(self):
        """Get FFmpeg command path."""
        if self.ffmpeg_path.exists():
            return str(self.ffmpeg_path)
        # Fallback to system ffmpeg
        return 'ffmpeg'
    
    def get_api_base_url(self):
        """Get API base URL."""
        return f"{self.api_url}/api/v1"